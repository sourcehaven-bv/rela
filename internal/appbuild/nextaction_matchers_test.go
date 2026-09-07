package appbuild

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func matcherMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"task": {Properties: map[string]metamodel.PropertyDef{
			"status":   {Type: metamodel.PropertyTypeString},
			"assignee": {Type: metamodel.PropertyTypeString},
			"watchers": {Type: metamodel.PropertyTypeString, List: true},
		}},
	}}
}

func matcherCfg(condition string) *dataentryconfig.Config {
	return &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "b"}},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"s": {Band: "b", Query: "type:task", Condition: condition, Suggest: "x"},
		},
	}
}

func stampedCtx(p principal.Principal) context.Context {
	return principal.With(context.Background(), p)
}

// TestQueryIdentityFor pins how the request principal becomes the query
// identity: nothing for an unstamped ctx or the "unknown" placeholder, the
// raw user otherwise, and the resolved entity id when the router resolved
// one (RawUser then holds the original).
func TestQueryIdentityFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		ctx    context.Context //nolint:containedctx // table fixture
		want   string
		wantOK bool
	}{
		{"unstamped", context.Background(), "", false},
		{"unknown placeholder", stampedCtx(principal.Principal{User: principal.Unknown, Tool: "data-entry"}), "", false},
		{"empty user", stampedCtx(principal.Principal{Tool: "data-entry"}), "", false},
		{"raw user", stampedCtx(principal.Principal{User: "alice@example.com", Tool: "data-entry"}), "alice@example.com", true},
		{"resolved user entity",
			stampedCtx(principal.Principal{User: "PERS-A", RawUser: "alice@example.com", Tool: "data-entry"}),
			"PERS-A", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, ok := queryIdentityFor(tc.ctx)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.want, q.ID())
		})
	}
}

// scoped applies the per-request scope binder the way the handler does.
func scoped(ctx context.Context, t *testing.T, scope func(context.Context) (context.Context, error)) context.Context {
	t.Helper()
	out, err := scope(ctx)
	require.NoError(t, err)
	return out
}

// TestNextActionMatchers_PrefiltersAndMatchShareTheIdentity is the contract
// dataentry builds on: the identity is stamped ONCE per request by the scope
// binder, and whatever the pre-filter pushes to the store is what the
// authoritative pass compares against.
func TestNextActionMatchers_PrefiltersAndMatchShareTheIdentity(t *testing.T) {
	t.Parallel()
	meta := matcherMeta()
	lookup, scope, problems := NextActionMatchers(
		matcherCfg("entity.status == 'open' and is_current_user(entity.assignee) and has_current_user(entity.watchers)"),
		meta)
	require.Empty(t, problems)
	require.NotNil(t, scope)
	m, ok := lookup("s")
	require.True(t, ok)
	pf, ok := m.(interface {
		Prefilters(context.Context, *metamodel.Metamodel) []store.PropPredicate
	})
	require.True(t, ok, "the matcher must offer the pre-filter capability dataentry looks for")

	alice := scoped(stampedCtx(principal.Principal{User: "alice", Tool: "data-entry"}), t, scope)
	require.Equal(t, []store.PropPredicate{
		{Property: "assignee", Op: store.PropEqual, Value: "alice", Scalar: true},
		{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
		{Property: "watchers", Op: store.PropEqual, Value: "alice", Scalar: false},
	}, pf.Prefilters(alice, meta))

	mine := entity.New("T-1", "task")
	mine.Properties["status"] = "open"
	mine.Properties["assignee"] = "alice"
	mine.Properties["watchers"] = []any{"bob", "alice"}
	got, err := m.Match(alice, mine)
	require.NoError(t, err)
	require.True(t, got)

	bob := scoped(stampedCtx(principal.Principal{User: "bob", Tool: "data-entry"}), t, scope)
	got, err = m.Match(bob, mine)
	require.NoError(t, err)
	require.False(t, got)

	// No identity (unstamped, or the placeholder): the current-user
	// conjuncts are not pushed (the literal still is), and the Go pass
	// refuses with the engine's sentinel.
	for _, ctx := range []context.Context{
		scoped(context.Background(), t, scope),
		scoped(stampedCtx(principal.Principal{User: principal.Unknown, Tool: "data-entry"}), t, scope),
	} {
		require.Equal(t, []store.PropPredicate{
			{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
		}, pf.Prefilters(ctx, meta))
		_, err = m.Match(ctx, mine)
		require.ErrorIs(t, err, nextaction.ErrIdentityRequired)
		require.ErrorIs(t, err, predicatefns.ErrNoCurrentUser)
	}

	// The matcher never derives on its own: an unscoped ctx with a principal
	// still has no identity. This is what makes "once per request" true.
	_, err = m.Match(stampedCtx(principal.Principal{User: "alice", Tool: "data-entry"}), mine)
	require.ErrorIs(t, err, nextaction.ErrIdentityRequired)
}

// TestNextActionRequestScope pins the scope binder: a boundary stamp is
// honored when it agrees with the principal (or there is none), derived
// from the principal otherwise, and REFUSED when the two disagree — two
// layers disagreeing about who is calling must never select either one's
// rows quietly.
func TestNextActionRequestScope(t *testing.T) {
	t.Parallel()
	aliceP := principal.Principal{User: "alice", Tool: "data-entry"}
	bobP := principal.Principal{User: "bob", Tool: "data-entry"}
	aliceQ := predicatefns.QueryIdentity{EntityID: "alice"}

	ctx, err := nextActionRequestScope(stampedCtx(aliceP))
	require.NoError(t, err)
	q, ok := predicatefns.QueryIdentityFrom(ctx)
	require.True(t, ok)
	require.Equal(t, "alice", q.ID())

	ctx, err = nextActionRequestScope(predicatefns.WithQueryIdentity(context.Background(), aliceQ))
	require.NoError(t, err)
	q, _ = predicatefns.QueryIdentityFrom(ctx)
	require.Equal(t, "alice", q.ID())

	ctx, err = nextActionRequestScope(predicatefns.WithQueryIdentity(stampedCtx(aliceP), aliceQ))
	require.NoError(t, err)
	q, _ = predicatefns.QueryIdentityFrom(ctx)
	require.Equal(t, "alice", q.ID())

	_, err = nextActionRequestScope(predicatefns.WithQueryIdentity(stampedCtx(bobP), aliceQ))
	require.ErrorIs(t, err, nextaction.ErrIdentityConflict)
	require.NotErrorIs(t, err, nextaction.ErrIdentityRequired,
		"a conflict is not an unauthenticated caller and must not be skipped as one")

	ctx, err = nextActionRequestScope(context.Background())
	require.NoError(t, err)
	_, ok = predicatefns.QueryIdentityFrom(ctx)
	require.False(t, ok)
}

func TestNextActionMatchers_IdentityFreeConditionWorksUnidentified(t *testing.T) {
	t.Parallel()
	lookup, scope, problems := NextActionMatchers(matcherCfg("entity.status == 'open'"), matcherMeta())
	require.Empty(t, problems)
	m, _ := lookup("s")
	e := entity.New("T-1", "task")
	e.Properties["status"] = "open"
	got, err := m.Match(scoped(context.Background(), t, scope), e)
	require.NoError(t, err)
	require.True(t, got)
}
