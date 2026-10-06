package appbuild

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// familyOnlyReader answers Family from a fixed table of readable faces. The
// embedded interface is nil: the gate under test calls Family only.
type familyOnlyReader struct {
	scriptEntityReaderFamily
	readable map[string][]entity.Face
}

func (r familyOnlyReader) Family(_ context.Context, id string) (visibility.Family, bool, error) {
	faces, ok := r.readable[id]
	if !ok {
		return visibility.Family{}, false, nil
	}
	return visibility.Family{ID: id, Faces: faces}, true, nil
}

// TestGatedGetRelation_TailFace pins the endpoint gate on a keyed relation
// read (TKT-KQXVF7). An identity edge needs any readable face of each
// endpoint. A content edge belongs to its tail face, so that face must be
// readable itself.
func TestGatedGetRelation_TailFace(t *testing.T) {
	ctx := context.Background()
	raw := memstore.New()
	pub := entity.Face("published")
	for _, e := range []*entity.Entity{
		{ID: "PAGE-1", Type: "page", Face: "draft"},
		{ID: "PAGE-1", Type: "page", Face: pub},
		{ID: "SPEC-1", Type: "spec"},
	} {
		require.NoError(t, raw.CreateEntity(ctx, e))
	}
	idKey := entity.RelationKey{From: "PAGE-1", Type: "references", To: "SPEC-1"}
	pubKey := entity.RelationKey{From: "PAGE-1", FromFace: pub, Type: "references", To: "SPEC-1"}
	for _, k := range []entity.RelationKey{idKey, pubKey} {
		_, err := raw.CreateRelation(ctx, k, nil)
		require.NoError(t, err)
	}

	tests := []struct {
		name     string
		readable map[string][]entity.Face
		key      entity.RelationKey
		wantOK   bool
	}{
		{"identity edge, draft face readable", map[string][]entity.Face{
			"PAGE-1": {"draft"}, "SPEC-1": {""},
		}, idKey, true},
		{"content edge, only draft face readable", map[string][]entity.Face{
			"PAGE-1": {"draft"}, "SPEC-1": {""},
		}, pubKey, false},
		{"content edge, tail face readable", map[string][]entity.Face{
			"PAGE-1": {pub}, "SPEC-1": {""},
		}, pubKey, true},
		{"target hidden", map[string][]entity.Face{
			"PAGE-1": {pub},
		}, pubKey, false},
		{"source hidden", map[string][]entity.Face{
			"SPEC-1": {""},
		}, idKey, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := gatedGraphReader{
				rows: familyOnlyReader{readable: tc.readable}, raw: raw, gateEndpoints: true,
			}
			r, err := g.GetRelation(ctx, tc.key)
			if !tc.wantOK {
				assert.ErrorIs(t, err, store.ErrNotFound)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.key, r.Identity())
		})
	}
}
