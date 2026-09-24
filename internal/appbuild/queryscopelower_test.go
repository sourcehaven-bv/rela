package appbuild_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const scopeLowerSchema = `version: "1.0"
entities:
  feature:
    label: Feature
    plural: features
    id_prefix: "FEAT-"
    id_type: sequential
    properties:
      title: {type: string}
    query_scopes:
      busy: "entity.title == 'x' and related(entity, 'implementedBy', { status = 'open' })"
      both: "related(entity, 'implementedBy') and related(entity, 'ownedBy')"
      out: "related(entity, 'tracks')"
      idle: "not related(entity, 'implementedBy')"
      twice: "related(entity, 'implementedBy') and related(entity, 'implementedBy')"
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      status: {type: string}
  user:
    label: User
    plural: users
    id_prefix: "USR-"
    id_type: sequential
    properties:
      title: {type: string}
relations:
  implements: {label: implements, inverse: implementedBy, from: [ticket], to: [feature]}
  owns: {label: owns, inverse: ownedBy, from: [user], to: [feature]}
  tracks: {label: tracks, from: [feature], to: [ticket]}
`

// lowerScope resolves a scope on feature and lowers it with gate.
func lowerScope(
	t *testing.T, scope string,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
) (store.GraphQuery, bool, bool) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(scopeLowerSchema))
	require.NoError(t, err)
	r, problems := appbuild.QueryScopes(nil, meta)
	require.Empty(t, problems)
	handle, _, ok := r.Resolve("feature", scope)
	require.True(t, ok)
	return r.Lower(context.Background(), handle, "feature", gate)
}

// gateBy answers each hop by its landing type: a nil error admits it with a
// predicate naming the type, so a test can see which entry came from where.
func gateBy(errs map[string]error) func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error) {
	return func(_ context.Context, _ string, hop acl.TraversalHop) (*store.RelationPredicate, error) {
		if err := errs[hop.EntityType]; err != nil {
			return nil, err
		}
		return &store.RelationPredicate{OfTypes: hop.RelationTypes, Endpoints: []string{hop.EntityType}}, nil
	}
}

func TestQueryScopeLower(t *testing.T) {
	denied := fmt.Errorf("wrapped: %w", acl.ErrTraversalDenied)
	refused := fmt.Errorf("wrapped: %w", acl.ErrTraversalUnsupported)

	t.Run("props and a gated incoming entry", func(t *testing.T) {
		frag, empty, ok := lowerScope(t, "busy", gateBy(nil))
		require.True(t, ok)
		require.False(t, empty)
		require.Equal(t, []store.PropPredicate{
			{Property: "title", Op: store.PropEqual, Value: "x", Scalar: true},
		}, frag.Props)
		require.Len(t, frag.Related, 1)
		require.True(t, frag.Related[0].Incoming)
		require.Equal(t, []string{"implements"}, frag.Related[0].Pred.OfTypes)
		require.Nil(t, frag.HasInbound, "a scope never writes the ACL's slot")
	})

	t.Run("an outgoing entry", func(t *testing.T) {
		frag, _, ok := lowerScope(t, "out", gateBy(nil))
		require.True(t, ok)
		require.Len(t, frag.Related, 1)
		require.False(t, frag.Related[0].Incoming)
	})

	t.Run("two incoming entries coexist", func(t *testing.T) {
		frag, _, ok := lowerScope(t, "both", gateBy(nil))
		require.True(t, ok)
		require.Len(t, frag.Related, 2)
	})

	t.Run("a denied traversal empties the page", func(t *testing.T) {
		frag, empty, ok := lowerScope(t, "both", gateBy(map[string]error{"user": denied}))
		require.True(t, ok)
		require.True(t, empty)
		require.Empty(t, frag.Related)
	})

	t.Run("a refused traversal falls back", func(t *testing.T) {
		_, _, ok := lowerScope(t, "both", gateBy(map[string]error{"user": refused}))
		require.False(t, ok)
	})

	t.Run("refused wins over denied", func(t *testing.T) {
		// The Go path reports the refusal as an error; "no match" from the
		// denied term must not hide it.
		_, _, ok := lowerScope(t, "both", gateBy(map[string]error{"ticket": denied, "user": refused}))
		require.False(t, ok)
	})

	t.Run("a scope that does not lower falls back without gating", func(t *testing.T) {
		gated := false
		_, _, ok := lowerScope(t, "idle", func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error) {
			gated = true
			return &store.RelationPredicate{}, nil
		})
		require.False(t, ok)
		require.False(t, gated)
	})

	t.Run("no gate falls back", func(t *testing.T) {
		_, _, ok := lowerScope(t, "busy", nil)
		require.False(t, ok)
	})

	t.Run("a nil predicate falls back rather than match every row", func(t *testing.T) {
		_, _, ok := lowerScope(t, "busy", func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error) {
			return nil, nil //nolint:nilnil // the malformed gate answer under test
		})
		require.False(t, ok)
	})

	t.Run("a repeated traversal is gated and joined once", func(t *testing.T) {
		calls := 0
		frag, _, ok := lowerScope(t, "twice", func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error) {
			calls++
			return &store.RelationPredicate{}, nil
		})
		require.True(t, ok)
		require.Equal(t, 1, calls)
		require.Len(t, frag.Related, 1)
	})
}
