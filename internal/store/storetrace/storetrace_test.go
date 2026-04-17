package storetrace_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetrace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ctx() context.Context { return context.Background() }

func seedGraph(t *testing.T) (*memstore.MemStore, *storetrace.GenericTracer) {
	t.Helper()
	s := memstore.New()

	// A --implements--> B --requires--> C
	// D (orphan)
	for _, e := range []*entity.Entity{
		func() *entity.Entity { e := entity.New("A", "decision"); e.SetString("title", "Decision A"); return e }(),
		func() *entity.Entity { e := entity.New("B", "requirement"); e.SetString("title", "Req B"); return e }(),
		func() *entity.Entity { e := entity.New("C", "requirement"); e.SetString("title", "Req C"); return e }(),
		func() *entity.Entity { e := entity.New("D", "component"); e.SetString("title", "Orphan D"); return e }(),
	} {
		require.NoError(t, s.CreateEntity(ctx(), e))
	}
	s.CreateRelation(ctx(), "A", "implements", "B", nil)
	s.CreateRelation(ctx(), "B", "requires", "C", nil)

	return s, storetrace.New(s)
}

func TestTraceFrom(t *testing.T) {
	_, tr := seedGraph(t)

	result := tr.TraceFrom(ctx(), "A", 0)
	require.NotNil(t, result)
	assert.Equal(t, "A", result.ID)
	assert.Equal(t, "decision", result.Type)
	assert.Equal(t, "Decision A", result.Title)

	// A has one outgoing edge to B
	require.Len(t, result.Children, 1)
	assert.Equal(t, "B", result.Children[0].ID)
	assert.Equal(t, "implements", result.Children[0].Relation)
	assert.False(t, result.Children[0].Incoming)

	// B has outgoing to C and incoming from A (A is visited, returned as leaf)
	bChildren := result.Children[0].Children
	require.Len(t, bChildren, 2)
	childIDs := map[string]bool{bChildren[0].ID: true, bChildren[1].ID: true}
	assert.True(t, childIDs["C"], "expected C in B's children")
	assert.True(t, childIDs["A"], "expected visited A leaf in B's children")
}

func TestTraceFrom_MaxDepth(t *testing.T) {
	_, tr := seedGraph(t)

	result := tr.TraceFrom(ctx(), "A", 1)
	require.NotNil(t, result)
	// depth 0 = A, depth 1 = B, but B's children at depth 2 are cut off
	require.Len(t, result.Children, 1)
	assert.Equal(t, "B", result.Children[0].ID)
	assert.Empty(t, result.Children[0].Children)
}

func TestTraceFrom_NotFound(t *testing.T) {
	_, tr := seedGraph(t)
	assert.Nil(t, tr.TraceFrom(ctx(), "NOPE", 0))
}

func TestTraceTo(t *testing.T) {
	_, tr := seedGraph(t)

	result := tr.TraceTo(ctx(), "C", 0)
	require.NotNil(t, result)
	assert.Equal(t, "C", result.ID)

	// C has incoming from B
	require.Len(t, result.Children, 1)
	assert.Equal(t, "B", result.Children[0].ID)

	// B has incoming from A
	require.Len(t, result.Children[0].Children, 1)
	assert.Equal(t, "A", result.Children[0].Children[0].ID)
}

func TestTraceTo_NoUpstream(t *testing.T) {
	_, tr := seedGraph(t)

	result := tr.TraceTo(ctx(), "A", 0)
	require.NotNil(t, result)
	assert.Empty(t, result.Children)
}

func TestFindPath(t *testing.T) {
	_, tr := seedGraph(t)

	path := tr.FindPath(ctx(), "A", "C")
	require.Len(t, path, 3)
	assert.Equal(t, "A", path[0].ID)
	assert.Equal(t, "B", path[1].ID)
	assert.Equal(t, "C", path[2].ID)
}

func TestFindPath_SameNode(t *testing.T) {
	_, tr := seedGraph(t)

	path := tr.FindPath(ctx(), "A", "A")
	require.Len(t, path, 1)
	assert.Equal(t, "A", path[0].ID)
}

func TestFindPath_Reverse(t *testing.T) {
	_, tr := seedGraph(t)

	// BFS treats graph as undirected — path C->A should work
	path := tr.FindPath(ctx(), "C", "A")
	require.Len(t, path, 3)
	assert.Equal(t, "C", path[0].ID)
	assert.Equal(t, "A", path[2].ID)
}

func TestFindPath_NoPath(t *testing.T) {
	_, tr := seedGraph(t)

	// D is an orphan, no path to A
	path := tr.FindPath(ctx(), "A", "D")
	assert.Nil(t, path)
}

func TestFindPath_NotFound(t *testing.T) {
	_, tr := seedGraph(t)
	assert.Nil(t, tr.FindPath(ctx(), "A", "NOPE"))
}

func TestFindOrphans(t *testing.T) {
	_, tr := seedGraph(t)

	orphans, err := tr.FindOrphans(ctx())
	require.NoError(t, err)
	require.Len(t, orphans, 1)
	assert.Equal(t, "D", orphans[0])
}

func TestHasCycle_NoCycle(t *testing.T) {
	_, tr := seedGraph(t)
	assert.False(t, tr.HasCycle(ctx(), "A"))
}

func TestHasCycle_WithCycle(t *testing.T) {
	s := memstore.New()
	s.CreateEntity(ctx(), entity.New("X", "t"))
	s.CreateEntity(ctx(), entity.New("Y", "t"))
	s.CreateRelation(ctx(), "X", "dep", "Y", nil)
	s.CreateRelation(ctx(), "Y", "dep", "X", nil)

	tr := storetrace.New(s)
	assert.True(t, tr.HasCycle(ctx(), "X"))
}
