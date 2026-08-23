//go:build sqlite

package cli

import (
	"errors"
	"fmt"
)

// The sqlite build has a real schema, so it must not inherit the
// "this binary has no database" stub — but the schema is created and kept
// current by sqlitestore.Open itself (CREATE TABLE IF NOT EXISTS), not by a
// versioned migration ladder like pgstore's.
//
// That difference is the whole content of these commands. Rather than
// pretending to a migration system this backend does not have, they say what
// is actually true: opening the project applies the schema, so there is
// nothing to run separately.
//
// If sqlitestore ever gains versioned migrations — which it will need the
// moment a released schema changes shape — these become real implementations
// mirroring db_postgres.go, and this comment is the marker for that work.

// errDBNoMigrationLadder explains why there is nothing to migrate.
var errDBNoMigrationLadder = errors.New(
	"the sqlite build applies its schema when the project is opened, so there " +
		"are no pending migrations to run; 'rela db status' reports the current state")

func runDBMigrate() error { return errDBNoMigrationLadder }

func runDBStatus() error {
	fmt.Println("Schema: applied at open (sqlite build).")
	fmt.Println("This backend has no versioned migration ladder; opening the")
	fmt.Println("project creates or updates the schema in place.")
	return nil
}

// runDBReconcile has no derived-schema objects to converge: the sqlite build
// inherits the non-postgres no-op reconciler, because `unique:` is enforced by
// the application-level check-then-write scan — which is correct here, since
// sqlitestore refuses a second process at Open (see DEC-LFSYNY).
func runDBReconcile(_, _ bool) error {
	return errors.New(
		"the sqlite build has no derived-schema objects to reconcile; " +
			"uniqueness is enforced by the application and by a single-writer lock")
}
