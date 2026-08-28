//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// startVersionSweepIfSupported is a no-op in non-postgres builds: only pgstore
// has a version-reconciliation sweep. fsstore already gets content versioning
// from git.
func startVersionSweepIfSupported(_ store.Store, _ *metamodel.Metamodel) {}

// versionServiceFor returns nil in non-postgres builds — content versioning is a
// pgstore-only service (fsstore uses git). Returning a GENUINELY nil interface
// (not a typed nil) is load-bearing: the entitymanager recorder factories and the
// service bundles nil-check this, and a typed-nil would defeat that check.
func versionServiceFor(_ store.Store) store.VersionService { return nil }

// stateKVFor returns nil in non-postgres builds: fsstore and memstore have no
// database to keep shared state in, so the caller falls back to the filesystem
// KV rooted at the project's .rela/ — which is correct for a single-process
// deployment. Genuinely nil (not a typed nil) so that nil-check works.
func stateKVFor(_ store.Store) state.KV { return nil }
