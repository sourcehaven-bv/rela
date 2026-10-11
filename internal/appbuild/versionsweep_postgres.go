//go:build postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// newBackendVersionSweep builds pgstore's reconciliation sweep over st's pool.
// A store that is not a *pgstore.Store gets no sweep, and a genuinely nil
// result rather than a typed nil, so the caller's nil check holds.
//
// Backend-specific on purpose: the sweep is pgstore's own service, not a store
// capability, so it is found by concrete type in the recipe's build rather than
// by an interface every backend would have to grow (TKT-IPXEZQ).
func newBackendVersionSweep(
	st store.Store, provider store.ProjectionProvider, cfg store.SweepConfig,
) (versionSweep, error) {
	ps, ok := st.(*pgstore.Store)
	if !ok {
		return nil, nil //nolint:nilnil // no pgstore means no sweep, which is not an error
	}
	sw, err := pgstore.NewVersionSweep(ps, provider, cfg)
	if err != nil {
		return nil, err
	}
	return sw, nil
}
