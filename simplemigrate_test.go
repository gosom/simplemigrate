package simplemigrate_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/gosom/simplemigrate"
	"github.com/gosom/simplemigrate/internal/mocks"
)

func Test_New(t *testing.T) {
	t.Parallel()

	t.Run("should return a migrator", func(t *testing.T) {
		t.Parallel()

		mctrl := gomock.NewController(t)
		defer mctrl.Finish()

		driver := mocks.NewMockDBDriver(mctrl)

		m := simplemigrate.New(driver)
		require.NotNil(t, m)
	})

	t.Run("should panic whem MigrationTable is empty", func(t *testing.T) {
		t.Parallel()

		mctrl := gomock.NewController(t)
		defer mctrl.Finish()

		driver := mocks.NewMockDBDriver(mctrl)

		require.Panics(t, func() {
			_ = simplemigrate.New(driver, simplemigrate.WithMigrationTable(""))
		})
	})

	t.Run("should panic when folder is empty", func(t *testing.T) {
		t.Parallel()

		driver := &mocks.MockDBDriver{}

		require.Panics(t, func() {
			_ = simplemigrate.New(driver, simplemigrate.WithSystemFS(""))
		})
	})
}

func Test_Migrate(t *testing.T) {
	t.Parallel()

	t.Run("happy path with filesystem and no pre-existing", func(t *testing.T) {
		t.Parallel()

		const (
			fname = "1_demo.sql"
			tbl   = "schema_migrations"
			stmt  = `CREATE TABLE demo (id INT NOT NULL);`
		)

		mctrl := gomock.NewController(t)
		defer mctrl.Finish()

		driver := mocks.NewMockDBDriver(mctrl)

		driver.EXPECT().CreateMigrationsTable(gomock.Any(), tbl).Return(nil)
		driver.EXPECT().SelectMigrations(gomock.Any(), tbl).Return(nil, nil)

		h := sha256.Sum256([]byte(stmt))

		m1 := simplemigrate.Migration{
			Version:    1,
			Fname:      fname,
			Hash:       fmt.Sprintf("%x", h),
			Statements: []string{stmt},
		}

		driver.EXPECT().ApplyMigrations(
			gomock.Any(),
			tbl,
			false,
			[]simplemigrate.Migration{m1},
		).
			Return(nil)

		m := simplemigrate.New(driver,
			simplemigrate.WithSystemFS("testdata/migrations"),
		)

		err := m.Migrate(context.Background())
		require.NoError(t, err)
	})

	t.Run("happy path with filesystem when already applied", func(t *testing.T) {
		t.Parallel()

		const (
			fname = "1_demo.sql"
			tbl   = "schema_migrations"
			stmt  = `CREATE TABLE demo (id INT NOT NULL);`
		)

		h := sha256.Sum256([]byte(stmt))

		m1 := simplemigrate.Migration{
			Version:    1,
			Fname:      fname,
			Hash:       fmt.Sprintf("%x", h),
			Statements: []string{stmt},
		}

		mctrl := gomock.NewController(t)
		defer mctrl.Finish()

		driver := mocks.NewMockDBDriver(mctrl)

		driver.EXPECT().CreateMigrationsTable(gomock.Any(), tbl).Return(nil)
		driver.EXPECT().SelectMigrations(gomock.Any(), tbl).
			Return([]simplemigrate.Migration{m1}, nil)

		m := simplemigrate.New(driver,
			simplemigrate.WithSystemFS("testdata/migrations"),
		)

		err := m.Migrate(context.Background())
		require.NoError(t, err)
	})

	t.Run("should return an error when driver.CreateMigrationsTable fails", func(t *testing.T) {
		t.Parallel()

		mctrl := gomock.NewController(t)
		defer mctrl.Finish()

		driver := mocks.NewMockDBDriver(mctrl)

		driver.EXPECT().CreateMigrationsTable(gomock.Any(), "schema_migrations").Return(errors.New("error"))

		m := simplemigrate.New(driver)

		err := m.Migrate(context.Background())
		require.Error(t, err)
	})
}
