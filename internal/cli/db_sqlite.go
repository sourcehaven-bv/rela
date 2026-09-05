//go:build sqlite

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// dbFileName duplicates appbuild's constant rather than importing it: the
// appbuild one is unexported, and exporting it to serve two CLI commands
// would widen a wiring package's API for a filename.
const dbFileName = "rela.db"

// databasePath locates the project's SQLite file the same way appbuild does.
func databasePath() (string, error) {
	paths, err := project.Discover("", storage.NewSafeFS(storage.NewOsFS()))
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.CacheDir, dbFileName), nil
}

// runDBMigrate carries the database forward to the shape this binary expects.
func runDBMigrate() error {
	path, err := databasePath()
	if err != nil {
		return err
	}
	ctx := context.Background()

	before, target, err := sqlitestore.Status(ctx, path)
	if err != nil {
		return err
	}
	if before >= target {
		fmt.Printf("Database is up to date (schema version %d).\n", before)
		return nil
	}

	// Connecting IS migrating: sqlitestore.Connect runs the ladder. Going
	// through it rather than exposing a separate migrate entry point keeps one
	// migration path, so an operator running this command and a server
	// starting up execute exactly the same code and cannot drift apart.
	conn, err := sqlitestore.Connect(ctx, sqlitestore.Options{Path: path})
	if err != nil {
		return err
	}
	if err := conn.Close(); err != nil {
		return err
	}

	// Report the target constant rather than re-reading. Connect succeeded, so
	// the database IS at that version; a third open would only add a TOCTOU
	// window and one more way for a command that already worked to return an
	// error.
	fmt.Printf("Applied migrations: schema version %d → %d.\n", before, target)
	return nil
}

// runDBStatus reports current vs expected schema version. Exits non-zero when
// the database is behind, so CI can gate on it — matching the postgres build.
//
// Status reads the stamp without migrating (it opens read-only and takes no
// single-writer lock), so "behind" is a state this can genuinely report — and
// reporting it does not require the server to be stopped.
func runDBStatus() error {
	path, err := databasePath()
	if err != nil {
		return err
	}
	current, target, err := sqlitestore.Status(context.Background(), path)
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

// runDBReconcile is a successful no-op: this build synthesizes no
// derived-schema objects, so there is nothing that could drift.
//
// Returning nil rather than an error is deliberate. `rela db reconcile
// --dry-run` is a documented CI drift gate on the postgres build, and a sqlite
// build in that same pipeline must not fail unconditionally.
func runDBReconcile(_, _ bool) error {
	fmt.Println("Nothing to reconcile: the sqlite build synthesizes no derived-schema")
	fmt.Println("objects. `unique:` is enforced by the application-level check, which")
	fmt.Println("is sound here because the store admits only one writer at a time.")
	return nil
}
