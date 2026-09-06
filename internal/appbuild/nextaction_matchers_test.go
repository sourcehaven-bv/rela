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

// TestNextActionMatchers_PrefiltersAndMatchShareTheIdentity is the contract
// dataentry builds on: whatever identity the pre-filter pushes to the store
// is the one the authoritative pass compares against.
func TestNextActionMatchers_PrefiltersAndMatchShareTheIdentity(t *testing.T) {
	t.Parallel()
	meta := matcherMeta()
	lookup, problems := NextActionMatchers(
		matcherCfg("entity.status == 'open' and is_current_user(entity.assignee) and has_current_user(entity.watchers)"),
		meta)
	require.Empty(t, problems)
	m, ok := lookup("s")
	require.True(t, ok)
	pf, ok := m.(interface {
		Prefilters(context.Context, *metamodel.Metamodel, []string) []store.PropPredicate
	})
	require.True(t, ok, "the matcher must offer the pre-filter capability dataentry looks for")

	alice := stampedCtx(principal.Principal{User: "alice", Tool: "data-entry"})
	require.Equal(t, []store.PropPredicate{
		{Property: "assignee", Op: store.PropEqual, Value: "alice", Scalar: true},
		{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
		{Property: "watchers", Op: store.PropEqual, Value: "alice", Scalar: false},
	}, pf.Prefilters(alice, meta, []string{"task"}))

	mine := entity.New("T-1", "task")
	mine.Properties["status"] = "open"
	mine.Properties["assignee"] = "alice"
	mine.Properties["watchers"] = []any{"bob", "alice"}
	got, err := m.Match(alice, mine)
	require.NoError(t, err)
	require.True(t, got)

	bob := stampedCtx(principal.Principal{User: "bob", Tool: "data-entry"})
	got, err = m.Match(bob, mine)
	require.NoError(t, err)
	require.False(t, got)

	// No identity: the current-user conjuncts are not pushed (the literal
	// still is), and the Go pass refuses with the engine's sentinel.
	require.Equal(t, []store.PropPredicate{
		{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
	}, pf.Prefilters(context.Background(), meta, []string{"task"}))
	_, err = m.Match(context.Background(), mine)
	require.ErrorIs(t, err, nextaction.ErrIdentityRequired)
	require.ErrorIs(t, err, predicatefns.ErrNoCurrentUser)

	// An identity already stamped by a boundary wins over the principal.
	pre := predicatefns.WithQueryIdentity(bob, predicatefns.QueryIdentity{EntityID: "alice"})
	got, err = m.Match(pre, mine)
	require.NoError(t, err)
	require.True(t, got)
	require.Equal(t, "alice", pf.Prefilters(pre, meta, []string{"task"})[0].Value)

	// No types: nothing to gate against, nothing pushed.
	require.Nil(t, pf.Prefilters(alice, meta, nil))
}

func TestNextActionMatchers_IdentityFreeConditionWorksUnidentified(t *testing.T) {
	t.Parallel()
	lookup, problems := NextActionMatchers(matcherCfg("entity.status == 'open'"), matcherMeta())
	require.Empty(t, problems)
	m, _ := lookup("s")
	e := entity.New("T-1", "task")
	e.Properties["status"] = "open"
	got, err := m.Match(context.Background(), e)
	require.NoError(t, err)
	require.True(t, got)
}
