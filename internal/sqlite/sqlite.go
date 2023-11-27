package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // sqlite driver

	"github.com/gosom/simplemigrate"
)

// driver is a struct that represents a sqlite driver
type driver struct {
	db *sql.DB
}

// New creates a new sqlite driver
func New(db *sql.DB) simplemigrate.DBDriver {
	return &driver{db: db}
}

// Connect connects to a sqlite database
func Connect(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

// Close closes the connection to the database
func (d *driver) Close(_ context.Context) error {
	return d.db.Close()
}

// Dialect returns the database dialect
func (d *driver) Dialect() string {
	return "sqlite"
}

// CreateMigrationsTable creates the migrations table
// If the table already exists, it does nothing
func (d *driver) CreateMigrationsTable(_ context.Context, migrationsTable string) error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS ` + migrationsTable + ` (
			version INTEGER NOT NULL PRIMARY KEY,
			fname TEXT NOT NULL,
			hash TEXT NOT NULL,
			applied_at DATETIME NOT NULL
		)
	`)

	return err
}

// SelectMigrations selects all migrations from the migrations table
// It returns a sorted slice (by Version ascending) of migrations or an error
func (d *driver) SelectMigrations(ctx context.Context, migrationsTable string) ([]simplemigrate.Migration, error) {
	//nolint:gosec // migrations table should be safe
	rows, err := d.db.QueryContext(ctx,
		"SELECT version, fname, hash, applied_at FROM "+migrationsTable+" ORDER BY version")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var migrations []simplemigrate.Migration

	for rows.Next() {
		var m simplemigrate.Migration

		var appliedAt string

		err := rows.Scan(&m.Version, &m.Fname, &m.Hash, &appliedAt)
		if err != nil {
			return nil, err
		}

		t, err := time.Parse(time.RFC3339Nano, appliedAt)
		if err != nil {
			return nil, err
		}

		m.AppliedAt = &t

		migrations = append(migrations, m)
	}

	return migrations, nil
}

// ApplyMigrations applies migrations to the database
// migrationsTable is the name of the migrations table
// If inTx is true, it applies all migrations in a transaction
// It returns an error if one occurs
func (d *driver) ApplyMigrations(ctx context.Context, migrationsTable string, inTx bool, migrations []simplemigrate.Migration) error {
	if inTx {
		fmt.Println("Applying migrations in transaction")

		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		defer func() {
			_ = tx.Rollback()
		}()

		err = d.applyMigrations(ctx, migrationsTable, tx, migrations)
		if err != nil {
			return err
		}

		return tx.Commit()
	}

	return d.applyMigrations(ctx, migrationsTable, nil, migrations)
}

func (d *driver) applyMigrations(ctx context.Context, migrationsTable string, tx *sql.Tx, migrations []simplemigrate.Migration) error {
	insertQ := "INSERT INTO " + migrationsTable + " (version, fname, hash, applied_at) VALUES (?, ?, ?, ?)"

	for _, m := range migrations {
		fmt.Printf("%s...", m.Fname)

		if err := d.applyOne(ctx, insertQ, tx, m); err != nil {
			fmt.Printf("FAILED\n")

			return err
		}

		fmt.Printf("OK\n")
	}

	return nil
}

func (d *driver) applyOne(ctx context.Context, insertQ string, tx *sql.Tx, m simplemigrate.Migration) error {
	trans, rollback, commit, err := d.createTxIfNotExists(ctx, tx)
	if err != nil {
		return err
	}

	defer func() {
		_ = rollback()
	}()

	for _, query := range m.Statements {
		_, err = trans.ExecContext(ctx, query)
		if err != nil {
			return err
		}
	}

	_, err = trans.ExecContext(ctx, insertQ, m.Version, m.Fname, m.Hash, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}

	err = commit()
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocritic // TODO: refactor
func (d *driver) createTxIfNotExists(
	ctx context.Context,
	tx *sql.Tx,
) (*sql.Tx, func() error, func() error, error) {
	if tx != nil {
		return tx, func() error { return nil }, func() error { return nil }, nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	return tx, tx.Rollback, tx.Commit, nil
}
