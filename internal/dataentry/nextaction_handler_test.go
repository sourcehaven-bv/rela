package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/userstate/memuserstate"
)

// withNextActions republishes the app's config with the given bands and
// sources, and gives it a fresh in-memory user-state store. Mirrors
// withListRenderOverride: copy the snapshot, mutate, publish.
func withNextActions(
	t *testing.T, app *App,
	bands []dataentryconfig.NextActionBand,
	sources map[string]dataentryconfig.NextActionSource,
) {
	t.Helper()
	s := app.State()
	cfg := *s.Cfg
	cfg.NextActionBands = bands
	cfg.NextActions = sources
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})

	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })
	require.NoError(t, app.SetUserState(st))
}

// seedTickets puts entities in the store: newTestAppV1 starts empty, so a
// query source has nothing to find until this runs.
func seedNextActionTickets(t *testing.T, app *App, ids ...string) {
	t.Helper()
	for _, id := range ids {
		seedEntity(app, &entity.Entity{
			ID: id, Type: "ticket",
			Properties: map[string]any{"title": "Ticket " + id, "status": "open"},
		})
	}
}

func getNextAction(ctx context.Context, t *testing.T, app *App) (resp nextActionResponse, status int) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_next_action", http.NoBody).WithContext(ctx)
	app.handleV1NextAction(rec, req)

	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	}
	return resp, rec.Code
}

func postFeedback(ctx context.Context, t *testing.T, app *App, body string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_next_action",
		strings.NewReader(body)).WithContext(ctx)
	app.handleV1NextAction(rec, req)
	return rec.Code
}

// A deployment with no next_actions must answer cleanly rather than 404 —
// the feature is opt-in and silence is its normal state.
func TestNextAction_UnconfiguredIsEmptyNot404(t *testing.T) {
	app := newTestAppV1(t)
	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, got.Suggestion)
}

func TestNextAction_CountSourceFiresOnEmptyType(t *testing.T) {
	app := newTestAppV1(t)
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "blocking"}},
		map[string]dataentryconfig.NextActionSource{
			"first-run": {
				Band:    "blocking",
				Count:   "nonexistent_type == 0",
				Suggest: "Nothing here yet.",
			},
		})

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, "first-run", got.Suggestion.Source)
	require.Empty(t, got.Suggestion.EntityID, "a count source names no entity")
}

// The type exists and has rows, so the first-run hint must stay quiet.
func TestNextAction_CountSourceSilentWhenTypePopulated(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "blocking"}},
		map[string]dataentryconfig.NextActionSource{
			"first-run": {Band: "blocking", Count: "ticket == 0", Suggest: "Start somewhere"},
		})

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, got.Suggestion)
}

func TestNextAction_QuerySourceInterpolates(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id} needs a look"},
		})

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.NotEmpty(t, got.Suggestion.EntityID)
	require.Contains(t, got.Suggestion.Message, got.Suggestion.EntityID,
		"the message should interpolate the candidate's id")
}

// Snoozing must actually suppress on the next GET — the whole point of the
// feedback endpoint.
func TestNextAction_SnoozeThenSuppressed(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id}"},
		})

	got, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, got.Suggestion)
	id := got.Suggestion.EntityID

	body := `{"source":"tickets","entity_id":"` + id + `","kind":"snooze","duration":"7d"}`
	require.Equal(t, http.StatusNoContent, postFeedback(aliceCtx(), t, app, body))

	after, _ := getNextAction(aliceCtx(), t, app)
	if after.Suggestion != nil {
		require.NotEqual(t, id, after.Suggestion.EntityID,
			"the snoozed entity must not be suggested again")
	}
}

func TestNextAction_MuteSilencesSource(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id}"},
		})

	got, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, got.Suggestion)

	require.Equal(t, http.StatusNoContent,
		postFeedback(aliceCtx(), t, app, `{"source":"tickets","kind":"mute"}`))

	after, _ := getNextAction(aliceCtx(), t, app)
	require.Nil(t, after.Suggestion, "a muted source must produce nothing")

	// And unmuting brings it back — reversibility is why per-source mute was
	// chosen over per-entity.
	require.Equal(t, http.StatusNoContent,
		postFeedback(aliceCtx(), t, app, `{"source":"tickets","kind":"unmute"}`))
	back, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, back.Suggestion, "unmute must restore the source")
}

// Per-user isolation: bob must not inherit alice's mute.
func TestNextAction_StateIsPerUser(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id}"},
		})

	require.Equal(t, http.StatusNoContent,
		postFeedback(principalCtx("alice"), t, app, `{"source":"tickets","kind":"mute"}`))

	aliceGot, _ := getNextAction(principalCtx("alice"), t, app)
	require.Nil(t, aliceGot.Suggestion)

	bobGot, _ := getNextAction(principalCtx("bob"), t, app)
	require.NotNil(t, bobGot.Suggestion, "bob must not inherit alice's mute")
}

// An unconfigured source id must be rejected: accepting one would let a
// request body create user-state rows keyed on nothing, and populate a mute
// list with ids that name no source and so can never be un-muted.
func TestNextAction_RejectsUnknownSource(t *testing.T) {
	app := newTestAppV1(t)
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id}"},
		})

	code := postFeedback(aliceCtx(), t, app, `{"source":"made-up","kind":"mute"}`)
	require.Equal(t, http.StatusBadRequest, code)
}

func TestNextAction_FeedbackValidation(t *testing.T) {
	app := newTestAppV1(t)
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id}"},
		})

	tests := []struct {
		name string
		body string
		want int
	}{
		{"malformed json", `{not json`, http.StatusBadRequest},
		{"missing source", `{"kind":"mute"}`, http.StatusBadRequest},
		{"unknown kind", `{"source":"tickets","kind":"explode"}`, http.StatusBadRequest},
		{"bad snooze duration", `{"source":"tickets","kind":"snooze","duration":"soon"}`, http.StatusBadRequest},
		{"valid mute", `{"source":"tickets","kind":"mute"}`, http.StatusNoContent},
		{"valid shown", `{"source":"tickets","kind":"shown"}`, http.StatusNoContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, postFeedback(aliceCtx(), t, app, tc.body))
		})
	}
}

func TestNextAction_MethodNotAllowed(t *testing.T) {
	app := newTestAppV1(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/_next_action", http.NoBody).
		WithContext(aliceCtx())
	app.handleV1NextAction(rec, req)
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// Resolving must not start the cooldown clock: only an explicit "shown"
// does. Otherwise a preview or a discarded response would silently consume
// the suggestion.
func TestNextAction_GetDoesNotStartCooldown(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id}", Cooldown: "7d"},
		})

	first, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, first.Suggestion)

	second, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, second.Suggestion, "a second GET without 'shown' must still suggest")
	require.Equal(t, first.Suggestion.EntityID, second.Suggestion.EntityID)
}

// countSourceConfig builds a first-run source over `ticket`.
func countSourceConfig(ungated bool) map[string]dataentryconfig.NextActionSource {
	return map[string]dataentryconfig.NextActionSource{
		"first-run": {
			Band:         "blocking",
			Count:        "ticket == 0",
			CountUngated: ungated,
			Suggest:      "Nothing here yet. Add a ticket?",
		},
	}
}

// TestNextAction_CountIsReadGatedByDefault is the disclosure guard.
//
// Tickets EXIST but bob may not read them. A gated count must therefore see
// none and fire the first-run hint for bob — mildly wrong, visibly wrong, and
// fixable in config. An UNGATED count would instead stay silent, and that
// silence tells bob that tickets exist, which is precisely the fact rela's
// read model treats as secret (a hidden entity is nonexistent).
func TestNextAction_CountIsReadGatedByDefault(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "blocking"}},
		countSourceConfig(false))

	// alice can read tickets, and one exists → no first-run hint.
	aliceGot, code := getNextAction(gateCtxFor(principalCtx("alice"), t, d), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, aliceGot.Suggestion, "alice sees a ticket, so first-run must stay quiet")

	// bob may read nothing → the gated count sees zero and fires.
	bobGot, code := getNextAction(gateCtxFor(principalCtx("bob"), t, d), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, bobGot.Suggestion,
		"a gated count must not disclose that tickets exist; it fires for a principal who sees none")
}

// The opt-out must be reachable, and must be the ONLY way to get the
// whole-graph answer.
func TestNextAction_CountUngatedIsOptIn(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "blocking"}},
		countSourceConfig(true))

	// With count_ungated, bob gets the whole-graph answer: a ticket exists,
	// so the hint stays quiet even though bob cannot see it.
	bobGot, code := getNextAction(gateCtxFor(principalCtx("bob"), t, d), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, bobGot.Suggestion, "count_ungated asks the whole-graph question")
}

// TestNextAction_VariantRoundTrips is the regression guard for a bug found in
// the browser: a source with key_props builds a suggestion key containing a
// Variant, but the GET response omitted it, so a client echoing back only
// (source, entity_id) stored its snooze under a DIFFERENT key. The POST
// returned 204 and the suggestion kept reappearing — a silent no-op.
//
// The round trip is the contract: whatever identifies the suggestion on the
// way out must be sufficient to suppress it on the way back in.
func TestNextAction_VariantRoundTrips(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {
				Band:     "stalled",
				Query:    "type:ticket",
				Suggest:  "{id}",
				KeyProps: []string{"status"},
			},
		})

	got, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, got.Suggestion)
	require.NotEmpty(t, got.Suggestion.Variant,
		"a source with key_props must expose the variant, or feedback cannot address the suggestion")

	// Echo back exactly what the GET provided.
	body := `{"source":"tickets","entity_id":"` + got.Suggestion.EntityID +
		`","variant":"` + got.Suggestion.Variant + `","kind":"snooze","duration":"7d"}`
	require.Equal(t, http.StatusNoContent, postFeedback(aliceCtx(), t, app, body))

	after, _ := getNextAction(aliceCtx(), t, app)
	require.Nil(t, after.Suggestion,
		"echoing the full key back must actually suppress the suggestion")
}

// TestQueryPushdown_MatchesGoSideFiltering is the parity guard for the
// property-predicate pushdown (Phase 1).
//
// executeQuery now asks the store to evaluate equality filters instead of
// loading a whole type and filtering in Go. That is only safe if both paths
// agree, so this drives a real query end-to-end through the next-action
// surface — a suggestion appearing for the wrong entity, or not at all, is
// exactly what a silent pushdown divergence looks like.
func TestQueryPushdown_MatchesGoSideFiltering(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-open", Type: "ticket",
		Properties: map[string]any{"title": "Open one", "status": "open"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-done", Type: "ticket",
		Properties: map[string]any{"title": "Done one", "status": "done"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-blank", Type: "ticket",
		Properties: map[string]any{"title": "No status"},
	})

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"equality pushes down", "type:ticket prop:status=open", []string{"TKT-open"}},
		{
			// An entity with no status must NOT satisfy an exclusion filter —
			// the asymmetry propmatch pins, and the one most likely to differ
			// between a SQL NOT and the Go rule.
			name:  "exclusion does not sweep in the unset row",
			query: "type:ticket prop:status!=open",
			want:  []string{"TKT-done"},
		},
		{"is-empty matches absent", "type:ticket prop:status=", []string{"TKT-blank"}},
		{
			// Rides on OpEqual but must stay in Go: pushed down it would
			// compare against the literal string "op*".
			name:  "glob still matches by pattern",
			query: "type:ticket prop:status=op*",
			want:  []string{"TKT-open"},
		},
		{"no property filter returns the type", "type:ticket", []string{"TKT-blank", "TKT-done", "TKT-open"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := app.executeQuery(aliceCtx(), tc.query)
			require.NoError(t, err)

			ids := make([]string, 0, len(got))
			for _, e := range got {
				ids = append(ids, e.ID)
			}
			sort.Strings(ids)
			require.Equal(t, tc.want, ids)
		})
	}
}

// TestNextAction_SourceScopedDeferIgnoresTheEchoedEntity pins that the SERVER
// owns the key shape.
//
// A client echoes back whatever the GET advertised, including entity_id — it
// needs that id to render a link. For a source-scoped source the key omits the
// entity, so trusting the echo verbatim stores the deferral under a key the
// engine never checks: a 204 that silently does nothing. That is exactly how
// this was found, walking the scenarios against a live server.
func TestNextAction_SourceScopedDeferIgnoresTheEchoedEntity(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001", "TKT-002")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {
				Band:       "stalled",
				Query:      "type:ticket",
				Suggest:    "{id}",
				DeferScope: dataentryconfig.DeferScopeSource,
			},
		})

	got, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, got.Suggestion)
	require.NotEmpty(t, got.Suggestion.EntityID, "the client still needs an id to link to")

	// Echo it back exactly as a client does.
	body := `{"source":"tickets","entity_id":"` + got.Suggestion.EntityID +
		`","kind":"snooze","duration":"7d"}`
	require.Equal(t, http.StatusNoContent, postFeedback(aliceCtx(), t, app, body))

	after, _ := getNextAction(aliceCtx(), t, app)
	require.Nil(t, after.Suggestion,
		"a source-scoped snooze must silence the source, not hand back its other candidate")
}

// The default stays per-entity: declining one item must not silence the rest.
func TestNextAction_EntityScopedDeferLeavesSiblings(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001", "TKT-002")
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "stalled"}},
		map[string]dataentryconfig.NextActionSource{
			"tickets": {Band: "stalled", Query: "type:ticket", Suggest: "{id}"},
		})

	got, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, got.Suggestion)
	first := got.Suggestion.EntityID

	body := `{"source":"tickets","entity_id":"` + first + `","kind":"snooze","duration":"7d"}`
	require.Equal(t, http.StatusNoContent, postFeedback(aliceCtx(), t, app, body))

	after, _ := getNextAction(aliceCtx(), t, app)
	require.NotNil(t, after.Suggestion, "the source must still offer its other candidate")
	require.NotEqual(t, first, after.Suggestion.EntityID)
}
