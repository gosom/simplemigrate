package simplemigrate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gosom/simplemigrate/internal/filesystem"
)

var (
	// ErrMigrationTableNameMissing is returned when the migrations table name is empty
	ErrMigrationTableNameMissing = errors.New("migrations table name cannot be empty")
	// ErrUnknownDriver is returned when the driver is unknown
	ErrUnknownDriver = errors.New("unknown driver")
	// ErrInvalidMigrationFile is returned when the migration file is invalid
	ErrInvalidMigrationFile = errors.New("invalid migration file")
	// ErrMigrationFolder is returned when the migration folder is invalid
	ErrMigrationFolder = errors.New("invalid migration folder")
	// ErrInvalidQuery is returned when the query is invalid
	ErrInvalidQuery = errors.New("invalid query")
)

const (
	// defaultMigrationsTable is the default name of the migrations table
	defaultMigrationsTable = "schema_migrations"
)

// Migration represents a single migration
type Migration struct {
	Version    int
	Fname      string
	AppliedAt  *time.Time
	Statements []string
	Hash       string
}

// DBDriver represents a database driver
//
//go:generate mockgen -destination=internal/mocks/mock_dbdriver.go -package=mocks . DBDriver
type DBDriver interface {
	// Dialect returns the database dialect
	Dialect() string
	// Close closes the connection to the database
	Close(ctx context.Context) error
	// CreateMigrationsTable creates the migrations table
	// migrationsTable is the name of the migrations table
	// If the table already exists, it does nothing
	// It returns an error if something goes wrong
	CreateMigrationsTable(ctx context.Context, migrationsTable string) error
	// SelectMigrations selects all migrations from the migrations table
	// migrationsTable is the name of the migrations table
	// It returns a sorted slice (by Version ascending) of migrations or an error
	SelectMigrations(ctx context.Context, migrationsTable string) ([]Migration, error)
	// ApplyMigrations applies migrations to the database
	// migrationsTable is the name of the migrations table
	// inTx is a flag that indicates if the migrations should be applied in a transaction
	// migrations is the slice of migrations to apply
	// It returns an error if something goes wrong
	ApplyMigrations(ctx context.Context, migrationsTable string, inTx bool, migrations []Migration) error
}

// QueryValidator represents a query validator
//
//go:generate mockgen -destination=internal/mocks/mock_queryvalidator.go -package=mocks . QueryValidator
type QueryValidator interface {
	ValidateQuery(ctx context.Context, dialect, query string) error
}

// Option represents a migrator option
type Option func(*Migrator) error

// Migrator is a struct that represents a migrator
// It is used to migrate a database
type Migrator struct {
	driver          DBDriver
	migrationsTable string
	printer         func(string, ...any)
	folder          fs.FS
	qvalidator      QueryValidator
	inTransaction   bool
}

// New is a constructor for Migrator
// Use this to create a new migrator
// and apply migrations to a database
// The driver is the database driver
// The options are used to configure the migrator
func New(driver DBDriver, opts ...Option) *Migrator {
	ans := Migrator{
		driver:          driver,
		migrationsTable: defaultMigrationsTable,
	}

	for _, opt := range opts {
		if err := opt(&ans); err != nil {
			panic(err)
		}
	}

	if ans.folder == nil {
		ans.folder = filesystem.NewSystemFS("migrations")
	}

	return &ans
}

// WithInTransaction is an option to apply all migrations in a transaction
// If an error occurs, the transaction is rolled back
// It is disabled by default
func WithInTransaction() Option {
	return func(m *Migrator) error {
		m.inTransaction = true

		return nil
	}
}

// WithQueryValidator is an option to enable query validation
// It is disabled by default
// Its purpose is to validate queries before applying them
func WithQueryValidator(validator QueryValidator) Option {
	return func(m *Migrator) error {
		m.qvalidator = validator

		return nil
	}
}

// WithSystemFS is an option to use the system filesystem
// The root is the root folder of the migrations
// It is "migrations" by default
func WithSystemFS(root string) Option {
	return func(m *Migrator) error {
		exists, err := isDir(root)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrMigrationFolder, err.Error())
		}

		if !exists {
			return fmt.Errorf("%w: %s", ErrMigrationFolder, root+" does not exist")
		}

		m.folder = filesystem.NewSystemFS(root)

		return nil
	}
}

// WithEmbedFS is an option to use the embed filesystem
// The fs is the embed filesystem
// It is nil by default
func WithEmbedFS(f fs.FS) Option {
	return func(m *Migrator) error {
		m.folder = f

		return nil
	}
}

// WithMigrationTable is an option to set the migrations table name
// It is "schema_migrations" by default
func WithMigrationTable(migrationsTable string) Option {
	return func(m *Migrator) error {
		if migrationsTable == "" {
			return ErrMigrationTableNameMissing
		}

		m.migrationsTable = migrationsTable

		return nil
	}
}

// Migrate is used to apply migrations to a database
// It returns an error if something goes wrong
func (m *Migrator) Migrate(ctx context.Context) error {
	fmt.Println("Migrating...")

	if err := m.driver.CreateMigrationsTable(ctx, m.migrationsTable); err != nil {
		return err
	}

	fmt.Println("Migrations table:", m.migrationsTable)

	localMigrations, err := m.readMigrations(ctx)
	if err != nil {
		return err
	}

	appliedMigrations, err := m.driver.SelectMigrations(ctx, m.migrationsTable)
	if err != nil {
		return err
	}

	if len(localMigrations) < len(appliedMigrations) {
		return fmt.Errorf("%w: %s", ErrInvalidMigrationFile, "local migrations are less than applied migrations")
	}

	for i := range appliedMigrations {
		if appliedMigrations[i].Version != localMigrations[i].Version {
			return fmt.Errorf("%w: %s", ErrInvalidMigrationFile, "local migrations are not in sync with applied migrations")
		}

		if appliedMigrations[i].Hash != localMigrations[i].Hash {
			return fmt.Errorf("%w: %s", ErrInvalidMigrationFile, "local migrations are not in sync with applied migrations")
		}
	}

	toApply := localMigrations[len(appliedMigrations):]

	if len(toApply) == 0 {
		fmt.Println("No migrations to apply")

		return nil
	}

	for _, migration := range toApply {
		if err := m.validate(ctx, migration); err != nil {
			return err
		}
	}

	fmt.Printf("Applying %d migrations [start_version=%d end_version=%d]\n",
		len(toApply), toApply[0].Version, toApply[len(toApply)-1].Version)

	return m.driver.ApplyMigrations(ctx, m.migrationsTable, m.inTransaction, toApply)
}

// readMigrations is used to read migrations from the filesystem
func (m *Migrator) readMigrations(_ context.Context) ([]Migration, error) {
	files, err := listFiles(m.folder, ".")
	if err != nil {
		return nil, err
	}

	items := make([]Migration, 0, len(files))

	for _, file := range files {
		migration := Migration{
			Fname: file,
		}

		idx := strings.Index(file, "_")
		if idx == -1 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMigrationFile, file+" must have a version")
		}

		if _, err := fmt.Sscanf(file[:idx], "%d", &migration.Version); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMigrationFile, file+" must have an integer version")
		}

		if migration.Version == 0 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMigrationFile, file+" must have a non-zero version")
		}

		data, err := fs.ReadFile(m.folder, file)
		if err != nil {
			return nil, err
		}

		data = bytes.TrimSpace(data)

		migration.Hash = computeHash(data)

		statements := strings.Split(string(data), "-- migrate:next")

		migration.Statements = statements

		items = append(items, migration)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Version < items[j].Version
	})

	if len(items) > 0 {
		if items[0].Version != 1 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMigrationFile, "first migration must have version 1")
		}

		for i := 1; i < len(items); i++ {
			if items[i].Version-items[i-1].Version != 1 {
				return nil, fmt.Errorf("%w: %s (%s - %s)", ErrInvalidMigrationFile, "migrations must have sequential versions", items[i-1].Fname, items[i].Fname)
			}
		}
	}

	return items, nil
}

// validate is used to validate a migration
func (m *Migrator) validate(ctx context.Context, migration Migration) error {
	if len(migration.Statements) == 0 {
		return fmt.Errorf("%w: %s", ErrInvalidMigrationFile, migration.Fname+" is empty")
	}

	if m.qvalidator != nil {
		for _, statement := range migration.Statements {
			if err := m.qvalidator.ValidateQuery(ctx, m.driver.Dialect(), statement); err != nil {
				return fmt.Errorf("%s: %w %s", migration.Fname, ErrInvalidQuery, err)
			}
		}
	}

	return nil
}

// listFiles is used to list files from the filesystem
func listFiles(fsys fs.FS, dir string) ([]string, error) {
	var files []string //nolint:prealloc // I don't know how many files are in the folder

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("%w: %s", ErrMigrationFolder, "cannot contain a folder")
		}

		if !strings.HasSuffix(entry.Name(), ".sql") {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMigrationFile, entry.Name()+" must have .sql extension")
		}

		files = append(files, entry.Name())
	}

	return files, nil
}

func isDir(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return fileInfo.IsDir(), nil
}

func computeHash(b []byte) string {
	hash := sha256.Sum256(b)

	return fmt.Sprintf("%x", hash)
}
