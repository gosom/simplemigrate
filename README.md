# simplemigrate

A simple, yet powerful, database migration library for Go. Designed for ease of use and safety in handling schema changes, `simplemigrate` currently supports SQLite, with plans to extend support to PostgreSQL and MySQL soon.

## Introduction

`simplemigrate` addresses common challenges encountered in team environments when managing database migrations. It ensures seamless integration of schema changes, preventing issues like duplicate version numbers or partially applied migrations.

## Key Features

- **Migrate Up Only**: Designed to only migrate up. Rolling back changes must be done manually, enhancing safety.
- **Sequential Versioning**: Migration versions must be sequential integers starting from 1, ensuring order and clarity.
- **No Duplicate Versions**: Duplicate version numbers in the migration folder are not allowed.
- **Migration Logging**: All applied migrations are logged with timestamps.
- **Transaction Support**: Capability to run all migrations within a single transaction.
- **Transactional SQL Statements**: Each SQL statement in a migration file is executed in a transaction. Multiple statements can be separated with `-- migrate:next`.
- **Library Usage**: Easily usable as a library in Go projects.
- **Query Validation**: Supports the ability to validate SQL statements before execution (WIP - not yet implemented).
- **Work-In-Progress**: This project is WIP, and currently only SQLite is supported. PostgreSQL and MySQL support are on the roadmap.

## Getting Started

### Installation

To install `simplemigrate`, use the following `go get` command:

```bash
go get github.com/gosom/simplemigrate
```

## Usage

### Command-Line Interface

Set the `DATABASE_URL` environment variable to your SQLite database connection string. Run migrations using the command-line interface:

```bash
simplemigrate -migrations-folder="path/to/migrations" -migrations-table-name="your_migrations_table"
```

other command line options:

```
  -enable-query-validation
        enables query validation (It's WIP - avoid USAGE)
  -migrations-folder string
        migrations folder (default "migrations")
  -migrations-table-name string
        migrations table name (default "schema_migrations")
  -transaction
        run all migrations in a transaction
```

### As a Library

Import `simplemigrate` into your Go project and use it to manage migrations:

```go
import "github.com/gosom/simplemigrate"

migrator := simplemigrate.New(driver, opts...)
err := migrator.Migrate(ctx)
```

## Configuration

`simplemigrate` can be configured with various options:

- `WithInTransaction`: Runs all migrations within a single transaction.
- `WithEnableQueryValidation`: Enables SQL query validation in migration files (not yet implemented).
- `WithSystemFS`: Uses the system filesystem for migration files.
- `WithEmbedFS`: Uses a embed file system (if you want to embed your migrations in the binary)
- `WithMigrationTable`: Change the default (schema_migrations) table name

## Contributing

Contributions to `simplemigrate` are welcome. Feel free to open issues or submit pull requests.

## License

`simplemigrate` is released under the MIT License. See the [LICENSE](LICENSE) file for more details.

## Contact

For questions and support, please open an issue in the GitHub repository.

