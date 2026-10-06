//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stateKVFor returns nil in non-postgres builds, so the caller falls back to
// the filesystem KV rooted at the project's .rela/. Genuinely nil (not a typed
// nil) so that nil-check works.
//
// fsstore and memstore have no database to keep shared state in. The sqlite
// recipe does not reach this fallback: it supplies the database KV through
// backendOverrides.stateKV (configloader_sqlite.go), because a self-contained
// document must carry its settings and document ID with it (FEAT-UP14BT).
func stateKVFor(_ store.Store) state.KV { return nil }
