//go:build sqlite

package cli

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// The sqlite build has a real schema, so it must not inherit the "this binary
// has no database" stub — but the schema is applied by sqlitestore.Open itself
// rather than by a migration ladder like pgstore's.
//
// It IS versioned: Open stamps PRAGMA user_version and refuses a database from
// a newer binary. What does not exist yet is the ladder that carries an OLDER
// database forward, which is why these commands report state rather than
// perform work. When schemaVersion is first bumped, that ladder becomes
// necessary and these become real implementations mirroring db_postgres.go.
//
// All three return nil for a no-op. `rela db migrate && start` is an ordinary
// deploy idiom and `rela db reconcile --dry-run` is a documented CI drift gate
// on the postgres build; a sqlite build in either pipeline must not fail just
// because it has nothing to do.

// runDBMigrate reports success, because on this build there is genuinely
// nothing to do — the schema is applied when the project is opened.
//
// Returning nil rather than an error is deliberate: `rela db migrate && start`
// is an ordinary deploy idiom, and failing a no-op would break it. The postgres
// equivalent does the same when the database is already current.
func runDBMigrate() error {
	fmt.Printf("Schema is up to date (version %d); the sqlite build applies it "+
		"when the project is opened.\n", sqlitestore.SchemaVersion())
	return nil
}

func runDBStatus() error {
	fmt.Printf("Schema version %d (sqlite build).\n", sqlitestore.SchemaVersion())
	fmt.Println("The schema is applied when the project is opened; a database")
	fmt.Println("written by a newer rela is refused rather than mis-read.")
	return nil
}

// runDBReconcile is a successful no-op for the same reason as runDBMigrate:
// `rela db reconcile --dry-run` is documented as a CI drift gate on the
// postgres build, and a sqlite build in that same pipeline must not fail
// unconditionally. There is no drift possible because there is nothing derived.
func runDBReconcile(_, _ bool) error {
	fmt.Println("Nothing to reconcile: the sqlite build synthesizes no derived-schema")
	fmt.Println("objects. `unique:` is enforced by the application-level check, which")
	fmt.Println("is sound here because the store admits only one writer at a time.")
	return nil
}
