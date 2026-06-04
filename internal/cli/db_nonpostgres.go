//go:build !postgres

package cli

import "errors"

// errDBNotAvailable is returned by `rela db` subcommands in non-postgres builds.
// The db command group only manages the PostgreSQL schema; the default
// (filesystem) and memorybackend builds have no database to migrate.
var errDBNotAvailable = errors.New(
	"the 'db' command requires the PostgreSQL build (rela-postgres); this binary uses the filesystem backend")

func runDBMigrate() error { return errDBNotAvailable }

func runDBStatus() error { return errDBNotAvailable }
