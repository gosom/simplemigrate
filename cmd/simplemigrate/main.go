package main

import (
	"context"
	"errors"
	"flag"
	"net/url"
	"os"

	"github.com/gosom/simplemigrate"
	"github.com/gosom/simplemigrate/internal/sqlfluff"
	"github.com/gosom/simplemigrate/internal/sqlite"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx); err != nil {
		panic(err)
	}
}

func run(ctx context.Context) error {
	args := parseArgs()

	driver, err := newDBDriver(ctx)
	if err != nil {
		return err
	}

	defer driver.Close(ctx)

	opts := []simplemigrate.Option{
		simplemigrate.WithSystemFS(args.migrationsFolder),
		simplemigrate.WithMigrationTable(args.migrationsTableName),
	}

	if args.enableQueryValidation {
		validator, err := sqlfluff.New()
		if err != nil {
			return err
		}

		opts = append(opts, simplemigrate.WithQueryValidator(validator))
	}

	if args.runInTransaction {
		opts = append(opts, simplemigrate.WithInTransaction())
	}

	migrator := simplemigrate.New(driver, opts...)

	return migrator.Migrate(ctx)
}

type args struct {
	runInTransaction      bool
	enableQueryValidation bool
	migrationsFolder      string
	migrationsTableName   string
}

func parseArgs() args {
	ans := args{}

	flag.BoolVar(&ans.runInTransaction, "transaction", false, "run all migrations in a transaction")
	flag.BoolVar(&ans.enableQueryValidation, "enable-query-validation", false, "enables query validation (It's WIP - avoid USAGE)")
	flag.StringVar(&ans.migrationsFolder, "migrations-folder", "migrations", "migrations folder")
	flag.StringVar(&ans.migrationsTableName, "migrations-table-name", "schema_migrations", "migrations table name")

	flag.Parse()

	return ans
}

func newDBDriver(ctx context.Context) (simplemigrate.DBDriver, error) {
	connURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		return nil, errors.New("DATABASE_URL is not set")
	}

	u, err := url.Parse(connURL)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "sqlite":
		conn, err := sqlite.Connect(u.Host)
		if err != nil {
			return nil, err
		}

		err = conn.PingContext(ctx)
		if err != nil {
			return nil, err
		}

		return sqlite.New(conn), nil
	default:
		return nil, simplemigrate.ErrUnknownDriver
	}
}
