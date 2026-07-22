//go:build postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// metaProjectionProvider yields the current render-schema projection from the
// metamodel, for the pgstore version sweep to stamp on create/update versions.
// It reads the metamodel on each call so a metamodel reload is picked up on the
// next sweep tick.
type metaProjectionProvider struct {
	meta *metamodel.Metamodel
}

func (p metaProjectionProvider) Projection() (hash string, projectionJSON []byte) {
	proj := p.meta.RenderProjection()
	b, err := proj.JSON()
	if err != nil {
		// Unreachable short of a runtime bug (RenderProjection is trivially
		// marshalable). Return an empty hash so the sweep tick skips capture
		// this round rather than stamping an empty projection.
		return "", nil
	}
	return proj.Hash(), b
}

// startVersionSweepIfSupported starts the pgstore reconciliation sweep. In the
// postgres build the store is a *pgstore.Store; a non-pgstore store (should not
// happen in this build) is left unswept.
func startVersionSweepIfSupported(st store.Store, meta *metamodel.Metamodel) {
	if s, ok := st.(*pgstore.Store); ok {
		s.StartVersionSweep(metaProjectionProvider{meta: meta}, pgstore.SweepConfig{})
	}
}

// versionServiceFor returns the pgstore versioning service (history reads, version
// writes, purge) sharing the store's pool. Returns a genuinely nil interface for a
// non-pgstore store (should not happen in this build), so nil-checks downstream
// behave correctly.
func versionServiceFor(st store.Store) store.VersionService {
	if s, ok := st.(*pgstore.Store); ok {
		return s.VersionStore()
	}
	return nil
}
