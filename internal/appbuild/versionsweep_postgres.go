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
	return proj.Hash(), proj.JSON()
}

// startVersionSweepIfSupported starts the pgstore reconciliation sweep. In the
// postgres build the store is a *pgstore.Store; a non-pgstore store (should not
// happen in this build) is left unswept.
func startVersionSweepIfSupported(st store.Store, meta *metamodel.Metamodel) {
	if s, ok := st.(*pgstore.Store); ok {
		s.StartVersionSweep(metaProjectionProvider{meta: meta}, pgstore.SweepConfig{})
	}
}
