//go:build sqlite

package appbuild

// Content versioning on the sqlite build (TKT-4NU9ZD).
//
// There is deliberately nothing in this file but its build tag, and that is
// the design rather than an oversight. Everything the wiring needs —
// startVersionSweepIfSupported, versionServiceFor, the two capability aliases,
// the projection provider and the cadence overrides — lives in
// versionsweep_shared.go, expressed entirely in store-package terms.
//
// That is what TKT-L3FNEN bought. The capabilities are discovered by type
// assertion on interfaces store declares (store.VersionSweeper,
// store.VersionServiceProvider), so a second backend implementing them needs
// no wiring code of its own: sqlitestore.Store satisfies both, and the shared
// resolvers find it exactly as they find pgstore's.
//
// The file exists so that the build tag stating "this build HAS versioning" is
// written down somewhere a reader will find it. Adding the tag to
// versionsweep_shared.go alone would work, but a future backend author looking
// for "where is sqlite's version wiring" would find nothing and reasonably
// conclude there is none — which is how the capability silently went unwired
// between the implementation landing and this file being written.
//
// What this build does NOT take from its database is shared state: the sqlite
// build keeps the filesystem KV on purpose (TKT-L1A3PH). See statekv_nodb.go
// for why the two capabilities go opposite ways.
