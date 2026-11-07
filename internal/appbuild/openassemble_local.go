//go:build !postgres

package appbuild

import (
	"context"
	"errors"
)

// openAndAssemble refuses on every build but postgres.
//
// The filesystem and sqlite builds keep their data in the project directory,
// which the shared config names, so "another" store would be the same one. The
// memory build opens a fresh store, but keeps piles, comments and settings
// under the project's cache directory. Either way every tenant would read and
// write one tenant's data. Refusing is the fail-closed choice, the same one
// `tenant.dsnForSchema` makes on these builds.
func openAndAssemble(context.Context, *SharedBase) (*Services, error) {
	return nil, errors.New(
		"appbuild: opening a store per DSN requires the postgres build; " +
			"the other builds keep their data in the project directory")
}
