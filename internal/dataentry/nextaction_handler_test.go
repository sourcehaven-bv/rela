package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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
