package appbuild_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// scopeTraversalSchema declares a feature scope that walks implements
// backwards, and one that uses the same traversal twice.
const scopeTraversalSchema = `version: "1.0"
entities:
  feature:
    label: Feature
    plural: features
    id_prefix: "FEAT-"
    id_type: sequential
    properties:
      title:
        type: string
    query_scopes:
      busy: "related(entity, 'implementedBy', { status = 'in-progress' })"
      idle: "not related(entity, 'implementedBy', { status = 'in-progress' })"
      twice: "related(entity, 'implementedBy', { status = 'in-progress' }) or (entity.title == 'x' and related(entity, 'implementedBy', { status = 'in-progress' }))"
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      status:
        type: string
relations:
  implements:
    label: implements
    inverse: implementedBy
    from: [ticket]
    to: [feature]
`

type traversalCalls struct {
	from    []string
	gates   []acl.TraversalHop
	queries []store.GraphQuery
}

func scopeFilterFixture(t *testing.T, scope string) func(gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
	match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
) ([]string, error) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(scopeTraversalSchema))
	require.NoError(t, err)
	r, problems := appbuild.QueryScopes(nil, meta)
	require.Empty(t, problems)
	handle, _, ok := r.Resolve("feature", scope)
	require.True(t, ok)
	headers := []store.EntityHeader{
		{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "a"}},
		{ID: "FEAT-2", Type: "feature", Properties: map[string]any{"title": "b"}},
	}
	return func(
		gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
		match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
	) ([]string, error) {
		got, err := r.Filter(context.Background(), handle, "feature", headers, gate, match)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(got))
		for _, h := range got {
			ids = append(ids, h.ID)
		}
		return ids, nil
	}
}

func (c *traversalCalls) gate(_ context.Context, from string, hop acl.TraversalHop) (*store.RelationPredicate, error) {
	c.from = append(c.from, from)
	c.gates = append(c.gates, hop)
	return acl.UngatedTraversal(hop)
}

func (c *traversalCalls) match(matching ...string) func(context.Context, store.GraphQuery, []string) (map[string]bool, error) {
	return func(_ context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
		c.queries = append(c.queries, q)
		out := make(map[string]bool, len(ids))
		for _, id := range ids {
			out[id] = false
		}
		for _, id := range matching {
			out[id] = true
		}
		return out, nil
	}
}

func TestQueryScopeFilter_AnswersAnIncomingTraversal(t *testing.T) {
	filter := scopeFilterFixture(t, "busy")
	var calls traversalCalls
	got, err := filter(calls.gate, calls.match("FEAT-2"))
	require.NoError(t, err)
	require.Equal(t, []string{"FEAT-2"}, got)

	require.Len(t, calls.gates, 1)
	require.Equal(t, []string{"feature"}, calls.from, "the gate must be told the candidate type")
	hop := calls.gates[0]
	require.True(t, hop.Incoming)
	require.Equal(t, []string{"implements"}, hop.RelationTypes)
	require.Equal(t, "ticket", hop.EntityType)
	require.Equal(t, []store.PropPredicate{{
		Property: "status", Op: store.PropEqual, Value: "in-progress", Scalar: true,
	}}, hop.Props)

	require.Len(t, calls.queries, 1)
	q := calls.queries[0]
	require.Equal(t, "feature", q.EntityType)
	require.Nil(t, q.HasOutbound)
	require.NotNil(t, q.HasInbound)
	require.Equal(t, "ticket", q.HasInbound.EndpointMatch.EntityType)
}

func TestQueryScopeFilter_OneQueryPerDistinctTraversal(t *testing.T) {
	filter := scopeFilterFixture(t, "twice")
	var calls traversalCalls
	got, err := filter(calls.gate, calls.match("FEAT-1"))
	require.NoError(t, err)
	require.Equal(t, []string{"FEAT-1"}, got)
	require.Len(t, calls.gates, 1)
	require.Len(t, calls.queries, 1)
}

func TestQueryScopeFilter_DeniedTraversalMatchesNothing(t *testing.T) {
	denied := func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error) {
		return nil, acl.ErrTraversalDenied
	}
	var calls traversalCalls

	got, err := scopeFilterFixture(t, "busy")(denied, calls.match("FEAT-1"))
	require.NoError(t, err)
	require.Empty(t, got)

	// A hidden entity is a nonexistent one, so the negation keeps every row.
	got, err = scopeFilterFixture(t, "idle")(denied, calls.match("FEAT-1"))
	require.NoError(t, err)
	require.Equal(t, []string{"FEAT-1", "FEAT-2"}, got)
	require.Empty(t, calls.queries, "a denied traversal must not reach the store")
}

func TestQueryScopeFilter_FailsClosed(t *testing.T) {
	boom := errors.New("boom")
	var calls traversalCalls
	cases := []struct {
		name  string
		gate  func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error)
		match func(context.Context, store.GraphQuery, []string) (map[string]bool, error)
		want  error
	}{
		{
			name: "unsupported gate",
			gate: func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error) {
				return nil, acl.ErrTraversalUnsupported
			},
			match: calls.match(),
			want:  acl.ErrTraversalUnsupported,
		},
		{
			name: "store error",
			gate: calls.gate,
			match: func(context.Context, store.GraphQuery, []string) (map[string]bool, error) {
				return nil, boom
			},
			want: boom,
		},
		{name: "no gate", match: calls.match()},
		{name: "no store", gate: calls.gate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The negated scope is the one a swallowed error would widen.
			got, err := scopeFilterFixture(t, "idle")(tc.gate, tc.match)
			require.Error(t, err)
			require.Nil(t, got)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}

func TestQueryScopeFilter_NoTraversalNeedsNoGate(t *testing.T) {
	meta, err := metamodel.Parse([]byte(scopeVisibilityMetamodel))
	require.NoError(t, err)
	r, problems := appbuild.QueryScopes(nil, meta)
	require.Empty(t, problems)
	handle, _, ok := r.Resolve("person", "named")
	require.True(t, ok)
	got, err := r.Filter(context.Background(), handle, "person", []store.EntityHeader{
		{ID: "PERS-1", Type: "person", Properties: map[string]any{"name": "bob"}},
		{ID: "PERS-2", Type: "person", Properties: map[string]any{"name": "eve"}},
	}, nil, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "PERS-1", got[0].ID)
}
