package worldreader_test

import (
	"context"
	"iter"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
	"github.com/Sourcehaven-BV/rela/internal/worldreader"
)

// filteringLister answers a RelationQuery the way a store would — by
// matching — so the batched and per-row paths can be compared on the
// same edge set.
type filteringLister struct {
	rels    []*entity.Relation
	queries int
}

func (l *filteringLister) ListRelations(_ context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	l.queries++
	match := storeutil.NewRelationMatcher(q)
	return func(yield func(*entity.Relation, error) bool) {
		for _, rel := range l.rels {
			if match(rel) && !yield(rel, nil) {
				return
			}
		}
	}
}

func face(t *testing.T, s string) entity.Face {
	t.Helper()
	f, err := entity.ParseFace(s)
	require.NoError(t, err)
	return f
}

// NeighborsForPage must equal Neighbors row by row — same edges, same order —
// while issuing one identity query plus one content query per distinct face
// on the page (TKT-1U8XYN).
func TestNeighborsForPage_EqualsPerRowNeighbors(t *testing.T) {
	published, nl := face(t, "published"), face(t, "nl")
	rel := func(from string, tail entity.Face, typ, to string) *entity.Relation {
		return &entity.Relation{From: from, FromFace: tail, Type: typ, To: to}
	}
	edges := []*entity.Relation{
		rel("POL-1", "", "owned-by", "TEAM-1"),         // identity, default tail
		rel("POL-1", published, "implements", "CTL-1"), // content, published tail
		rel("POL-1", "", "implements", "CTL-2"),        // content, draft tail (not POL-1's face on this page)
		rel("POL-2", "", "implements", "CTL-3"),        // content, default tail — POL-2's face
		rel("POL-2", "", "owned-by", "TEAM-2"),         // identity
		rel("DOC-1", nl, "references", "POL-1"),        // content from DOC-1's nl face to a page row
		rel("DOC-1", "", "references", "POL-2"),        // content from DOC-1's en face — not its face here
		rel("POL-2", published, "implements", "POL-1"), // content edge between two rows, published tail
		rel("X-9", "", "owned-by", "POL-1"),            // identity, incoming to a row
	}
	classes := contentTypes{"implements": true, "references": true}
	rows := []worldreader.Resolved{
		{Entity: &entity.Entity{ID: "POL-1", Type: "policy", Face: published}, Face: published, Found: true},
		{Entity: &entity.Entity{ID: "POL-2", Type: "policy"}, Face: "", Found: true},
		{Entity: &entity.Entity{ID: "DOC-1", Type: "document", Face: nl}, Face: nl, Found: true},
		{Found: false}, // excluded by the world: no edges
	}
	for _, dir := range []store.Direction{store.DirectionBoth, store.DirectionOutgoing, store.DirectionIncoming} {
		lister := &filteringLister{rels: edges}
		rr, err := worldreader.NewRelationReader(lister, classes)
		require.NoError(t, err)

		got, err := rr.NeighborsForPage(context.Background(), rows, dir)
		require.NoError(t, err)
		require.Len(t, got, len(rows))
		// identity query + one content query per distinct face (published, "", nl)
		require.Equal(t, 4, lister.queries, "direction %v", dir)

		for i, res := range rows {
			want, err := rr.Neighbors(context.Background(), res, dir)
			require.NoError(t, err)
			require.Equal(t, keys(want), keys(got[i]), "direction %v row %d", dir, i)
		}
	}
}

func TestNeighborsForPage_EmptyPage(t *testing.T) {
	lister := &filteringLister{}
	rr, err := worldreader.NewRelationReader(lister, contentTypes{})
	require.NoError(t, err)
	got, err := rr.NeighborsForPage(context.Background(), nil, store.DirectionBoth)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Zero(t, lister.queries, "nothing to ask for")
}

func keys(rels []*entity.Relation) []string {
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, r.Key())
	}
	return out
}
