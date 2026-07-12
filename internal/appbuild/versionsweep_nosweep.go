//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// startVersionSweepIfSupported is a no-op in non-postgres builds: only pgstore
// has a version-reconciliation sweep. fsstore already gets content versioning
// from git.
func startVersionSweepIfSupported(_ store.Store, _ *metamodel.Metamodel) {}
