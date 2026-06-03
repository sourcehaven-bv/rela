//go:build postgres

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// resolveDSN returns the database URL from the --database-url flag / env (kong
// populates databaseURL from both), erroring if none is set.
func resolveDSN(dsn string) (string, error) {
	if dsn == "" {
		return "", errors.New("no database URL: set --database-url or RELA_DATABASE_URL")
	}
	return dsn, nil
}

// runDBMigrate applies pending migrations (postgres build). Pool construction
// lives in pgstore (MigrateDSN/StatusDSN) so the CLI doesn't depend on pgx.
func runDBMigrate(dsn string) error {
	resolved, err := resolveDSN(dsn)
	if err != nil {
		return err
	}
	ctx := context.Background()
	before, target, err := pgstore.StatusDSN(ctx, resolved)
	if err != nil {
		return err
	}
	if before >= target {
		fmt.Printf("Database is up to date (schema version %d).\n", before)
		return nil
	}
	if err := pgstore.MigrateDSN(ctx, resolved); err != nil {
		return err
	}
	fmt.Printf("Applied migrations: schema version %d → %d.\n", before, target)
	return nil
}

// runDBStatus reports current vs target schema version. Exits non-zero when the
// database is behind, so CI can gate on it.
func runDBStatus(dsn string) error {
	resolved, err := resolveDSN(dsn)
	if err != nil {
		return err
	}
	current, target, err := pgstore.StatusDSN(context.Background(), resolved)
	if err != nil {
		return err
	}
	if current < target {
		fmt.Printf("Database is BEHIND: schema version %d, binary expects %d.\n", current, target)
		fmt.Println("Run 'rela db migrate' to apply pending migrations.")
		os.Exit(1)
	}
	fmt.Printf("Database is up to date (schema version %d).\n", current)
	return nil
}
