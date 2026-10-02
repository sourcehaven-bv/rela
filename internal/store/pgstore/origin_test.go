package pgstore_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// copyOrigin is the provenance a copy definition would stamp: this test drives
// the STORE directly (it is a pgstore test, not an entitymanager one), so it
// supplies the ctx value the entitymanager write boundary supplies in
// production. The route is identical — store.WithOrigin on ctx — which is the
// point: there is no second way in.
var copyOrigin = store.Origin{
	Kind:       store.OriginCopy,
	Source:     "POL-1",
	SourceFace: "draft",
	SourceType: "policy",
	Definition: "publish",
}

// The sweep's copy-origin capture is pinned for every versioning backend by
// storetest.RunSweepOriginTests; this file keeps the synchronous path.

// TestSyncCaptureCarriesOrigin covers the OTHER capture path. create/update
// arrive via the sweep; a synchronous capture carries provenance
// inside store.VersionInput instead, and the two must agree — a field wired
// through only one path is worse than one wired through neither, because the
// timeline silently changes meaning depending on which op produced the row.
func TestSyncCaptureCarriesOrigin(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	require.NoError(t, s.VersionStore().WriteVersion(ctx, store.VersionInput{
		EntityID:   "POL-5",
		Op:         store.VersionOpDelete,
		Type:       "policy",
		Content:    "final body",
		SchemaHash: "schema-abc",
		Projection: []byte(`{"entities":{},"types":{}}`),
		Origin:     copyOrigin,
	}))

	versions, err := s.VersionStore().ListVersions(ctx, entity.Ref{ID: "POL-5"})
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.Equal(t, copyOrigin, versions[0].Origin)
}
