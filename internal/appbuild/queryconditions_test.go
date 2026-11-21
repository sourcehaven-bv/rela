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
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
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

	filter, err := conds.Lower(context.Background(), prog, "feature")
	require.Error(t, err, "current_user lowers only with a stamped identity")
	require.Empty(t, filter.Props)
	require.Empty(t, filter.Traversals)

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	ctx, err = conds.StampQueryIdentity(ctx)
	require.NoError(t, err)
	filter, err = conds.Lower(ctx, prog, "feature")
	require.NoError(t, err)
	require.Equal(t, []store.PropPredicate{
		{Property: "title", Op: store.PropEqual, Value: "alice", Scalar: true},
	}, filter.Props)
	require.Nil(t, filter.Where, "a plain conjunction needs no tree")
	require.Equal(t, []visibility.PageTraversal{{
		Hop: acl.TraversalHop{
			RelationTypes: []string{"implements"}, Incoming: true, EntityType: "ticket", Props: []store.PropPredicate{},
		},
		Negate: true,
	}}, filter.Traversals)

	// An `or` rides in Where, and every field it reads is one of its props.
	orProg, err := ev.CompileWithCurrentUser("feature", "entity.title == 'a' or entity.title < 'c'")
	require.NoError(t, err)
	filter, err = conds.Lower(ctx, orProg, "feature")
	require.NoError(t, err)
	require.Empty(t, filter.Props)
	require.NotNil(t, filter.Where)
	require.Equal(t, store.CondOr, filter.Where.Op)
	require.Len(t, filter.Where.Props(), 2)

	filter, err = appbuild.QueryConditions{}.Lower(ctx, prog, "feature")
	require.Error(t, err, "the zero value lowers nothing")
	require.Empty(t, filter.Props)
	require.Empty(t, filter.Traversals)

	// A refusal names the part that does not lower.
	twoProps, err := ev.CompileWithCurrentUser("feature", "entity.title == 'a' or entity.title < entity.title")
	require.NoError(t, err)
	_, err = conds.Lower(ctx, twoProps, "feature")
	var unsupported *queryplan.UnsupportedConditionError
	require.ErrorAs(t, err, &unsupported)
	require.Contains(t, err.Error(), "entity.title < entity.title")
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
		// Inside a Where tree a denied traversal is false (TKT-8HY05WB).
		{"bob", "related(entity, 'implementedBy') or entity.title == 'b'", []string{"FEAT-1", "FEAT-2"}},
		{"alice", "related(entity, 'implementedBy') or entity.title == 'b'", []string{"FEAT-2"}},
		{"bob", "not (related(entity, 'implementedBy') and entity.title == 'a')", []string{"FEAT-2"}},
		{"alice", "not (related(entity, 'implementedBy') and entity.title == 'a')", []string{"FEAT-1", "FEAT-2"}},
	} {
		t.Run(tc.user+" "+tc.filter, func(t *testing.T) {
			t.Parallel()
			callCtx := principal.With(ctx, principal.Principal{User: tc.user, Tool: principal.ToolMCP})
			callCtx, err := appbuild.MustIdentityBinder(svc)(callCtx)
			require.NoError(t, err)
			prog, err := ev.Compile("feature", tc.filter)
			require.NoError(t, err)
			filter, err := reads.Conditions.Lower(callCtx, prog, "feature")
			require.NoError(t, err)
			filter.Type, filter.Faces, filter.Limit = "feature", store.AllFaces(), 10
			page, err := reads.Reader.ListEntityPage(callCtx, filter)
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
