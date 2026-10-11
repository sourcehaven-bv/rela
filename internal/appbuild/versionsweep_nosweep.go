//go:build !postgres && !sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// startVersionSweepIfSupported is a no-op in builds with no versioning
// backend: only pgstore and sqlitestore have a version-reconciliation sweep.
// fsstore already gets content versioning from git, and memstore keeps nothing
// past the process.
func startVersionSweepIfSupported(_ store.Store, _ *metamodel.Metamodel) {}

// versionServiceFor returns nil here — content versioning is a database-backed
// service (fsstore uses git). Returning a GENUINELY nil interface (not a typed
// nil) is load-bearing: the entitymanager recorder factories and the service
// bundles nil-check this, and a typed-nil would defeat that check.
func versionServiceFor(_ store.Store) store.VersionService { return nil }

// versionTaggerFor returns nil here: version tags live beside the database
// version history, which this build does not have.
func versionTaggerFor(_ store.Store, _ *metamodel.Metamodel) (store.VersionTagger, error) {
	return nil, nil //nolint:nilnil // no history means no tagger, which is not an error
}

// versionTagReaderFor returns nil here, for the reason versionTaggerFor does.
func versionTagReaderFor(_ store.Store) visibility.VersionTagReader { return nil }
