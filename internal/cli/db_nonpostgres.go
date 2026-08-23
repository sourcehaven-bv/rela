//go:build !postgres && !sqlite

package cli

import "errors"

// errDBNotAvailable is returned by `rela db` subcommands in builds with no
// database to manage — the default (filesystem) and memorybackend builds.
//
// Tagged `!postgres && !sqlite` rather than just `!postgres`: the sqlite build
// DOES have a schema, and inheriting this file would tell its users to switch
// to PostgreSQL and claim they are on the filesystem backend. Both wrong.
var errDBNotAvailable = errors.New(
	"the 'db' command manages a database schema; this binary uses the " +
		"filesystem backend and has none. Use the PostgreSQL build " +
		"(rela-postgres) or the SQLite build (rela-sqlite) if you need it")

func runDBMigrate() error { return errDBNotAvailable }

func runDBStatus() error { return errDBNotAvailable }

func runDBReconcile(_, _ bool) error { return errDBNotAvailable }
