package tracer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/tracer/tracertest"
)

// A faced type has no row at the bare id, so a tracer that read only the
// default face lost it from every trace, path and orphan report
// (BUG-95W7MV). Nodes are families: one node per id, listing its faces.
func seedFaced(t *testing.T) *memstore.MemStore {
	t.Helper()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "Pub"}},
		{ID: "POL-1", Type: "policy", Face: "draft", Properties: map[string]any{"title": "Draft"}},
		{ID: "POL-2", Type: "policy", Face: "draft"},
		{ID: "CTL-1", Type: "control", Properties: map[string]any{"title": "Control"}},
		{ID: "NOTE-1", Type: "note"},
	} {
		require.NoError(t, st.CreateEntity(ctx(), e))
	}
	_, err := st.CreateRelation(ctx(), entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "implements", To: "CTL-1"}, &store.RelationData{})
	require.NoError(t, err)
	return st
}

func TestTrace_FacedNodeIsAFamily(t *testing.T) {
	st := seedFaced(t)
	tr := tracertest.Must(st, store.TrivialScope())

	res := tr.TraceFrom(ctx(), "POL-1", 2)
	require.NotNil(t, res, "a faced entity must trace")
	assert.Equal(t, []entity.Face{"draft", "published"}, res.Faces)
	require.Len(t, res.Children, 1)
	assert.Equal(t, "CTL-1", res.Children[0].ID)
	assert.Nil(t, res.Children[0].Faces, "a faceless node lists no faces")
	assert.Equal(t, tracer.Tail{ID: "POL-1", Face: "draft"}, res.Children[0].Tail)

	up := tr.TraceTo(ctx(), "CTL-1", 2)
	require.NotNil(t, up)
	require.Len(t, up.Children, 1)
	assert.Equal(t, []entity.Face{"draft", "published"}, up.Children[0].Faces)

	path := tr.FindPath(ctx(), "CTL-1", "POL-1")
	require.Len(t, path, 2)
	assert.Equal(t, []entity.Face{"draft", "published"}, path[1].Faces)
}

// The world picks the face whose title a node shows; the face list is the
// same in every world.
func TestTrace_WorldSelectsTheServedFace(t *testing.T) {
	st := seedFaced(t)
	w := store.NewWorldScope(map[string]store.TypeResolution{"policy": {Chain: []entity.Face{"published", "draft"}}})
	res := tracertest.Must(st, w).TraceFrom(ctx(), "POL-1", 1)
	require.NotNil(t, res)
	assert.Equal(t, "Pub", res.Title)
	assert.Equal(t, []entity.Face{"draft", "published"}, res.Faces)
}

func TestFindOrphans_FacedFamilies(t *testing.T) {
	st := seedFaced(t)
	got, err := tracertest.Must(st, store.TrivialScope()).FindOrphans(ctx())
	require.NoError(t, err)
	assert.Equal(t, []tracer.Orphan{
		{ID: "NOTE-1", Type: "note"},
		{ID: "POL-2", Type: "policy", Faces: []entity.Face{"draft"}},
	}, got)
}

// An edge tailed on a face with no row connects nothing: the tail does not
// exist.
func TestFindOrphans_EdgeOnMissingFaceConnectsNothing(t *testing.T) {
	st := memstore.New()
	require.NoError(t, st.CreateEntity(ctx(), &entity.Entity{ID: "POL-1", Type: "policy", Face: "draft"}))
	require.NoError(t, st.CreateEntity(ctx(), &entity.Entity{ID: "CTL-1", Type: "control"}))
	_, err := st.CreateRelation(ctx(), entity.RelationKey{From: "POL-1", FromFace: "published", Type: "implements", To: "CTL-1"}, &store.RelationData{})
	require.NoError(t, err)

	got, err := tracertest.Must(st, store.TrivialScope()).FindOrphans(ctx())
	require.NoError(t, err)
	assert.Len(t, got, 2)
}
