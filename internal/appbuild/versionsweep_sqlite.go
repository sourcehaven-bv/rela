//go:build sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// Content versioning on the sqlite build (TKT-4NU9ZD).
//
// Everything backend-neutral the wiring needs (versionServiceFor, the
// projection provider, the cadence overrides and newVersionSweep) lives in
// versionsweep_shared.go, expressed in store-package terms. The version
// service is discovered by type assertion on store.VersionServiceProvider, so
// sqlitestore needs no wiring code for it.
//
// The sweep is the exception. It is sqlitestore's own service rather than a
// store capability (TKT-IPXEZQ), so this file builds it by concrete type, the
// same way versionsweep_postgres.go builds pgstore's.
//
// What this build does NOT take from its database is shared state: the sqlite
// build keeps the filesystem KV on purpose (TKT-L1A3PH). See statekv_nodb.go
// for why the two capabilities go opposite ways.

// newBackendVersionSweep builds sqlitestore's reconciliation sweep over st. A
// store that is not a *sqlitestore.Store gets no sweep, and a genuinely nil
// result rather than a typed nil, so the caller's nil check holds.
func newBackendVersionSweep(
	st store.Store, provider store.ProjectionProvider, cfg store.SweepConfig,
) (versionSweep, error) {
	ss, ok := st.(*sqlitestore.Store)
	if !ok {
		return nil, nil //nolint:nilnil // no sqlitestore means no sweep, which is not an error
	}
	sw, err := sqlitestore.NewVersionSweep(ss, provider, cfg)
	if err != nil {
		return nil, err
	}
	return sw, nil
}
