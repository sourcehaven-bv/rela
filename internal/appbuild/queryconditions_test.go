package appbuild_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// GatedReads' Conditions lowers a condition to props and ungated hops, and
// binds current_user from the stamped principal (TKT-Z099FGY). The zero
// value lowers nothing.
func TestQueryConditions_Lower(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(scopeLowerSchema))
	require.NoError(t, err)
	svc := appbuildtest.New(meta)
	t.Cleanup(func() { _ = svc.Close() })
	conds := svc.GatedReads().Conditions
	ev := predicatefns.NewEvaluator(meta)

	prog, err := ev.CompileWithCurrentUser("feature",
		"entity.title == current_user.id and not related(entity, 'implementedBy')")
	require.NoError(t, err)

	props, traversals, ok := conds.Lower(context.Background(), prog, "feature")
	require.False(t, ok, "current_user lowers only with a stamped identity")
	require.Empty(t, props)
	require.Empty(t, traversals)

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	ctx, err = conds.StampQueryIdentity(ctx)
	require.NoError(t, err)
	props, traversals, ok = conds.Lower(ctx, prog, "feature")
	require.True(t, ok)
	require.Equal(t, []store.PropPredicate{
		{Property: "title", Op: store.PropEqual, Value: "alice", Scalar: true},
	}, props)
	require.Equal(t, []visibility.PageTraversal{{
		Hop: acl.TraversalHop{
			RelationTypes: []string{"implements"}, Incoming: true, EntityType: "ticket", Props: []store.PropPredicate{},
		},
		Negate: true,
	}}, traversals)

	props, traversals, ok = appbuild.QueryConditions{}.Lower(ctx, prog, "feature")
	require.False(t, ok, "the zero value lowers nothing")
	require.Empty(t, props)
	require.Empty(t, traversals)
}

// The bundle's Reader gates the traversals Conditions returns with the
// caller's own policy: for a caller who may not read tickets, a required
// traversal onto them matches nothing and a negated one narrows nothing
// (TKT-Z099FGY).
func TestGatedReads_ConditionTraversalsGatedByReader(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(scopeLowerSchema))
	require.NoError(t, err)
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "a"}},
		{ID: "FEAT-2", Type: "feature", Properties: map[string]any{"title": "b"}},
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"status": "open"}},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	_, err = st.CreateRelation(ctx, entity.RelationKey{From: "TKT-1", Type: "implements", To: "FEAT-1"}, nil)
	require.NoError(t, err)
	var policy acl.Policy
	require.NoError(t, yaml.Unmarshal([]byte(`
roles:
  everything:
    read: ["*"]
  featureOnly:
    read: [feature, user]
assignments:
  alice: featureOnly
  bob: everything
`), &policy))
	d, err := acl.NewDeclarative(&policy, aclstore.MustNewGraph(st), st)
	require.NoError(t, err)
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })
	reads := svc.GatedReads()
	ev := predicatefns.NewEvaluator(meta)

	for _, tc := range []struct {
		user, filter string
		want         []string
	}{
		{"bob", "related(entity, 'implementedBy')", []string{"FEAT-1"}},
		{"bob", "not related(entity, 'implementedBy')", []string{"FEAT-2"}},
		{"alice", "related(entity, 'implementedBy')", nil},
		{"alice", "not related(entity, 'implementedBy')", []string{"FEAT-1", "FEAT-2"}},
	} {
		t.Run(tc.user+" "+tc.filter, func(t *testing.T) {
			t.Parallel()
			callCtx := principal.With(ctx, principal.Principal{User: tc.user, Tool: principal.ToolMCP})
			callCtx, err := appbuild.MustIdentityBinder(svc)(callCtx)
			require.NoError(t, err)
			prog, err := ev.Compile("feature", tc.filter)
			require.NoError(t, err)
			props, traversals, ok := reads.Conditions.Lower(callCtx, prog, "feature")
			require.True(t, ok)
			page, err := reads.Reader.ListEntityPage(callCtx, visibility.PageQuery{
				Type: "feature", Faces: store.AllFaces(), Props: props, Traversals: traversals, Limit: 10,
			})
			require.NoError(t, err)
			var ids []string
			for _, e := range page.Rows {
				ids = append(ids, e.ID)
			}
			require.True(t, slices.Equal(ids, tc.want), "rows %v, want %v", ids, tc.want)
			require.Equal(t, len(tc.want), page.Total)
		})
	}
}
