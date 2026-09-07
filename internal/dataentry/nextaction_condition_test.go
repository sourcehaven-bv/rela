package dataentry

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// withTicketAssignment republishes the app's metamodel with the two
// properties a per-user condition needs on `ticket`: a scalar `assignee`
// and a list `watchers`. Copies the snapshot rather than mutating the
// shared fixture, exactly as withNextActions does for the config.
func withTicketAssignment(t *testing.T, app *App) {
	t.Helper()
	s := app.State()
	meta := *s.Meta
	meta.Entities = maps.Clone(meta.Entities)
	def := meta.Entities["ticket"]
	def.Properties = maps.Clone(def.Properties)
	def.Properties["assignee"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	def.Properties["watchers"] = metamodel.PropertyDef{Type: metamodel.PropertyTypeString, List: true}
	meta.Entities["ticket"] = def
	app.schema.Publish(&Schema{
		Cfg: s.Cfg, Meta: &meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})
}

func seedAssignedTicket(app *App, id, assignee string, watchers ...string) {
	props := map[string]any{"title": "Ticket " + id, "status": "open"}
	if assignee != "" {
		props["assignee"] = assignee
	}
	if len(watchers) > 0 {
		w := make([]any, 0, len(watchers))
		for _, x := range watchers {
			w = append(w, x)
		}
		props["watchers"] = w
	}
	seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: props})
}

var oneBand = []dataentryconfig.NextActionBand{{ID: "b"}}

// recordingMatcher is a fake condition matcher: it pushes whatever
// predicates the test hands it and answers Match with a fixed verdict,
// counting how often it was asked to pre-filter.
type recordingMatcher struct {
	push  []store.PropPredicate
	match bool
	calls int
}

func (m *recordingMatcher) Match(context.Context, *entity.Entity) (bool, error) {
	return m.match, nil
}

func (m *recordingMatcher) Prefilters(context.Context, *metamodel.Metamodel) []store.PropPredicate {
	m.calls++
	return m.push
}

// doNextActionGet is getNextAction without the JSON decode, for asserting
// on an error body.
func doNextActionGet(ctx context.Context, t *testing.T, app *App) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_next_action", http.NoBody).WithContext(ctx)
	app.handleV1NextAction(rec, req)
	return rec
}

func matcherFuncFor(m nextaction.Matcher) NextActionMatcherFunc {
	return func(
		*dataentryconfig.Config, *metamodel.Metamodel,
	) (func(string) (nextaction.Matcher, bool), func(context.Context) (context.Context, error), []string) {
		return func(string) (nextaction.Matcher, bool) { return m, true }, nil, nil
	}
}

// TestNextAction_ConditionPrefiltersReachTheStore proves the plumbing, not
// the predicate semantics: a matcher's pre-filter is applied by the STORE
// before the engine sees candidates, and the engine's own pass stays
// authoritative over whatever the pre-filter let through.
func TestNextAction_ConditionPrefiltersReachTheStore(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	seedAssignedTicket(app, "TKT-mine", "alice")
	seedAssignedTicket(app, "TKT-theirs", "bob")
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"s": {Band: "b", Query: "type:ticket", Condition: "irrelevant-to-the-fake", Suggest: "{id}"},
	})

	t.Run("the pre-filter narrows the fetch", func(t *testing.T) {
		// Match accepts EVERYTHING, so the only way TKT-theirs can be
		// absent is the predicate having reached the store.
		m := &recordingMatcher{
			push:  []store.PropPredicate{{Property: "assignee", Op: store.PropEqual, Value: "alice", Scalar: true}},
			match: true,
		}
		require.NoError(t, app.SetNextActionMatchers(matcherFuncFor(m)))
		resp, status := getNextAction(aliceCtx(), t, app)
		require.Equal(t, http.StatusOK, status)
		require.NotNil(t, resp.Suggestion)
		require.Equal(t, "TKT-mine", resp.Suggestion.EntityID)
		require.Equal(t, 1, m.calls, "pre-filtered exactly once for the source")
	})

	t.Run("the Go pass stays authoritative", func(t *testing.T) {
		// The pre-filter keeps TKT-mine; Match rejects it anyway.
		m := &recordingMatcher{
			push:  []store.PropPredicate{{Property: "assignee", Op: store.PropEqual, Value: "alice", Scalar: true}},
			match: false,
		}
		require.NoError(t, app.SetNextActionMatchers(matcherFuncFor(m)))
		resp, status := getNextAction(aliceCtx(), t, app)
		require.Equal(t, http.StatusOK, status)
		require.Nil(t, resp.Suggestion)
	})

	t.Run("no pre-filter falls back to the unpushed query", func(t *testing.T) {
		m := &recordingMatcher{match: true}
		require.NoError(t, app.SetNextActionMatchers(matcherFuncFor(m)))
		_, status := getNextAction(aliceCtx(), t, app)
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, 1, m.calls)
	})
}

// TestNextAction_CurrentUserConditionEndToEnd runs the real composition-root
// matcher (appbuild.NextActionMatchers) behind the handler: the condition
// compiles with current_user, the identity comes from the request
// principal, and each principal is offered only their own ticket.
func TestNextAction_CurrentUserConditionEndToEnd(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	seedAssignedTicket(app, "TKT-alice", "alice")
	seedAssignedTicket(app, "TKT-bob", "bob")
	seedAssignedTicket(app, "TKT-watched", "carol", "dave", "alice")
	require.NoError(t, app.SetNextActionMatchers(appbuild.NextActionMatchers))

	t.Run("the composition-root matcher offers the pre-filter capability", func(t *testing.T) {
		lookup, scope, problems := appbuild.NextActionMatchers(&dataentryconfig.Config{
			NextActions: map[string]dataentryconfig.NextActionSource{
				"s": {Query: "type:ticket", Condition: "is_current_user(entity.assignee)"},
			},
		}, app.State().Meta)
		require.Empty(t, problems)
		require.NotNil(t, scope, "appbuild supplies the per-request scope binder")
		m, ok := lookup("s")
		require.True(t, ok)
		_, ok = m.(ConditionPrefilterer)
		require.True(t, ok, "appbuild's matcher must satisfy dataentry.ConditionPrefilterer")
	})

	cases := []struct {
		name      string
		condition string
		user      string
		want      string
	}{
		{"equality against current_user.id", "entity.assignee == current_user.id", "alice", "TKT-alice"},
		{"is_current_user picks alice's", "is_current_user(entity.assignee)", "alice", "TKT-alice"},
		{"is_current_user picks bob's", "is_current_user(entity.assignee)", "bob", "TKT-bob"},
		{"has_current_user finds the watcher", "has_current_user(entity.watchers)", "alice", "TKT-watched"},
		{"has_current_user for a non-watcher finds nothing", "has_current_user(entity.watchers)", "bob", ""},
		{"an OR still evaluates correctly, just unpushed",
			"is_current_user(entity.assignee) or has_current_user(entity.watchers)", "dave", "TKT-watched"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
				"s": {Band: "b", Query: "type:ticket", Condition: tc.condition, Suggest: "{id}"},
			})
			resp, status := getNextAction(principalCtx(tc.user), t, app)
			require.Equal(t, http.StatusOK, status)
			if tc.want == "" {
				require.Nil(t, resp.Suggestion)
				return
			}
			require.NotNil(t, resp.Suggestion)
			require.Equal(t, tc.want, resp.Suggestion.EntityID)
		})
	}
}

// TestNextAction_CurrentUserConditionWithoutIdentityIsNamed pins the
// fail-closed direction at the HTTP layer: a per-user condition on a request
// with no identity (the "unknown" placeholder the server stamps without an
// identity source) is refused with an error that names the misconfiguration
// — never answered with nobody's rows, and never dressed as "search failed".
func TestNextAction_CurrentUserConditionWithoutIdentityIsNamed(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	seedAssignedTicket(app, "TKT-alice", "alice")
	require.NoError(t, app.SetNextActionMatchers(appbuild.NextActionMatchers))
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"s": {Band: "b", Query: "type:ticket", Condition: "is_current_user(entity.assignee)", Suggest: "{id}"},
	})

	for _, ctx := range []context.Context{context.Background(), principalCtx(principal.Unknown)} {
		rec := doNextActionGet(ctx, t, app)
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.Contains(t, rec.Body.String(), "next_action_identity_required")
	}

	// A condition that never mentions the user is unaffected on the same
	// unidentified request.
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"s": {Band: "b", Query: "type:ticket", Condition: "entity.status == 'open'", Suggest: "{id}"},
	})
	resp, status := getNextAction(context.Background(), t, app)
	require.Equal(t, http.StatusOK, status)
	require.NotNil(t, resp.Suggestion)
}

// TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse pins the
// raw-vs-redacted asymmetry documented on ConditionPrefilterer: the store
// pre-filter keeps alice's ticket (it compares the raw assignee), but the
// authoritative Go pass evaluates the REDACTED candidate, where a
// visible:-hidden assignee binds Nil and is_current_user is false. The Go
// pass is therefore strictly narrower, and a hidden value can never drive a
// suggestion.
func TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	seedAssignedTicket(app, "TKT-alice", "alice")
	require.NoError(t, app.SetNextActionMatchers(appbuild.NextActionMatchers))
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"s": {Band: "b", Query: "type:ticket", Condition: "is_current_user(entity.assignee)", Suggest: "{id}"},
	})
	ctx := gateCtxFor(principalCtx("alice"), t, d)

	// Baseline: with the assignee visible the suggestion fires.
	resp, status := getNextAction(ctx, t, app)
	require.Equal(t, http.StatusOK, status)
	require.NotNil(t, resp.Suggestion)
	require.Equal(t, "TKT-alice", resp.Suggestion.EntityID)

	// Hide the assignee: the pre-filter still selects the row, the Go pass
	// does not.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"assignee": false}}}
	resp, status = getNextAction(ctx, t, app)
	require.Equal(t, http.StatusOK, status)
	require.Nil(t, resp.Suggestion, "a hidden property must not satisfy a per-user condition")
}
