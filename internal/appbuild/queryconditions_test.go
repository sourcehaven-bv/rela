package appbuild_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// GatedReads' Conditions lowers a condition with the bundle's traversal gate
// and binds current_user from the stamped principal (TKT-Z099FGY). The zero
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

	props, related, empty, ok := conds.Lower(context.Background(), prog, "feature")
	require.False(t, ok, "current_user lowers only with a stamped identity")
	require.False(t, empty)
	require.Empty(t, props)
	require.Empty(t, related)

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	ctx, err = conds.StampQueryIdentity(ctx)
	require.NoError(t, err)
	props, related, empty, ok = conds.Lower(ctx, prog, "feature")
	require.True(t, ok)
	require.False(t, empty)
	require.Equal(t, []store.PropPredicate{
		{Property: "title", Op: store.PropEqual, Value: "alice", Scalar: true},
	}, props)
	require.Len(t, related, 1)
	require.True(t, related[0].Incoming)
	require.True(t, related[0].Pred.Negate)

	props, related, empty, ok = appbuild.QueryConditions{}.Lower(ctx, prog, "feature")
	require.False(t, ok, "the zero value lowers nothing")
	require.False(t, empty)
	require.Empty(t, props)
	require.Empty(t, related)
}

// A traversal onto a type the caller may not read is answered by the
// bundle's own policy: a positive one empties the result, a negated one
// drops out (TKT-Z099FGY).
func TestQueryConditions_LowerDeniedTraversal(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(scopeLowerSchema))
	require.NoError(t, err)
	st := memstore.New()
	var policy acl.Policy
	require.NoError(t, yaml.Unmarshal([]byte(`
roles:
  featureOnly:
    read: [feature, user]
assignments:
  alice: featureOnly
`), &policy))
	d, err := acl.NewDeclarative(&policy, aclstore.MustNewGraph(st), st)
	require.NoError(t, err)
	svc := appbuildtest.New(meta, appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })
	conds := svc.GatedReads().Conditions

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	ctx, err = appbuild.MustIdentityBinder(svc)(ctx)
	require.NoError(t, err)
	ev := predicatefns.NewEvaluator(meta)

	positive, err := ev.Compile("feature", "related(entity, 'implementedBy')")
	require.NoError(t, err)
	props, related, empty, ok := conds.Lower(ctx, positive, "feature")
	require.True(t, ok)
	require.True(t, empty, "a required traversal onto an unreadable type matches nothing")
	require.Empty(t, props)
	require.Empty(t, related)

	negated, err := ev.Compile("feature", "not related(entity, 'implementedBy') and related(entity, 'ownedBy')")
	require.NoError(t, err)
	_, related, empty, ok = conds.Lower(ctx, negated, "feature")
	require.True(t, ok)
	require.False(t, empty)
	require.Len(t, related, 1, "the denied negated traversal drops out")
	require.False(t, related[0].Pred.Negate)
}
