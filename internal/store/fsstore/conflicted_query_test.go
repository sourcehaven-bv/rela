package fsstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// conflictedTicketStore seeds tickets T-1 and T-2 with title "open", an edge
// T-2 --blocks--> R-1, and then rewrites T-2's file with conflict markers.
// The file stays in the index, but its values cannot be read.
func conflictedTicketStore(t *testing.T) *FSStore {
	t.Helper()
	s, fs := newTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"T-1", "T-2"} {
		e := entity.New(id, "ticket")
		e.Properties = map[string]any{"title": "open"}
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	require.NoError(t, s.CreateEntity(ctx, entity.New("R-1", "requirement")))
	_, err := s.CreateRelation(ctx, entity.RelationKey{From: "T-2", Type: "blocks", To: "R-1"}, nil)
	require.NoError(t, err)
	conflicted := "---\nid: T-2\ntype: ticket\n<<<<<<< HEAD\ntitle: open\n=======\ntitle: closed\n>>>>>>> theirs\n---\n"
	require.NoError(t, fs.WriteFileExternal("/entities/tickets/T-2.md", []byte(conflicted), 0o644))
	return s
}

func collectIDs(t *testing.T, s *FSStore, q store.GraphQuery) ([]string, error) {
	t.Helper()
	var got []string
	for e, err := range s.GraphQuery(context.Background(), q) {
		if err != nil {
			return got, err
		}
		got = append(got, e.ID)
	}
	return got, nil
}

// TestGraphQuery_SkipUnreadable pins BUG-HTSW37Q. The ACL read-grant query
// sets SkipUnreadable, so a conflicted candidate row is hidden instead of
// failing every list of its type. Every other query still fails, and so does
// an endpoint match, because a left-out row there would read as "not
// related" and widen a negated predicate.
func TestGraphQuery_SkipUnreadable(t *testing.T) {
	s := conflictedTicketStore(t)
	open := []store.PropPredicate{{Property: "title", Op: store.PropEqual, Value: "open"}}

	t.Run("read gate hides the conflicted row", func(t *testing.T) {
		q := store.GraphQuery{EntityType: "ticket", Props: open, Faces: store.AllFaces(), SkipUnreadable: true}
		got, err := collectIDs(t, s, q)
		require.NoError(t, err)
		require.Equal(t, []string{"T-1"}, got)

		faces, err := s.MatchingFaces(context.Background(), q, []string{"T-1", "T-2"})
		require.NoError(t, err)
		require.Contains(t, faces, "T-1")
		require.NotContains(t, faces, "T-2")
	})

	t.Run("other queries fail closed", func(t *testing.T) {
		_, err := collectIDs(t, s, store.GraphQuery{EntityType: "ticket", Props: open, Faces: store.AllFaces()})
		require.ErrorIs(t, err, store.ErrConflictedRow)
	})

	t.Run("a negated endpoint match fails closed", func(t *testing.T) {
		q := store.GraphQuery{
			EntityType: "requirement", Faces: store.AllFaces(), SkipUnreadable: true,
			HasInbound: &store.RelationPredicate{
				OfTypes: []string{"blocks"}, Negate: true,
				EndpointMatch: &store.EndpointPredicate{EntityType: "ticket", Props: open},
			},
		}
		got, err := collectIDs(t, s, q)
		require.ErrorIs(t, err, store.ErrConflictedRow)
		require.Empty(t, got, "R-1 must not read as unblocked")
	})
}
