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
// fsstore and memstore have no database to keep shared state in, which settles
// it for them. The SQLITE build inherits the filesystem KV deliberately rather
// than for want of a table (TKT-L1A3PH): node-local state is only a problem
// when several processes serve one project — the documented failure is an
// operator's logo upload landing on one node while the others keep serving the
// old one — and sqlitestore is single-process by construction, so "node-local"
// and "the only node" coincide.
//
// Note this is the one capability the sqlite build does NOT take from its
// database even though it could. Versioning went the other way (see
// versionsweep_sqlite.go), and the difference is the point: versioning has to
// be in the database because history must survive alongside the rows it
// describes, while shared state buys nothing a single process can observe.
func stateKVFor(_ store.Store) state.KV { return nil }
