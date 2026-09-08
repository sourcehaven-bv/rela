package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The next-action surface is world-scoped on TWO independent axes:
//
//   - source_world   — the world a source's QUERY runs in (what is FOUND).
//   - visible_worlds — an allow list of worlds it may be DISPLAYED in.
//
// These tests keep them separated. The whole point of two keys is that a
// source may query `published` while remaining visible everywhere, so a test
// that only ever set both together would pass against an implementation that
// conflated them.

// seedFacedTicket seeds a ticket with a DEFAULT-face row and, when
// publishedTitle is non-empty, a `published` face carrying different content.
// The two titles differ so a test can tell WHICH face a suggestion
// interpolated.
func seedFacedTicket(t *testing.T, app *App, id, draftTitle, publishedTitle string) {
	t.Helper()
	seedEntity(app, &entity.Entity{
		ID: id, Type: "ticket",
		Properties: map[string]any{"title": draftTitle, "status": "open"},
	})
	if publishedTitle == "" {
		return
	}
	require.NoError(t, app.store.CreateEntity(t.Context(), &entity.Entity{
		ID: id, Type: "ticket", Face: entity.Face("published"),
		Properties: map[string]any{"title": publishedTitle, "status": "open"},
	}))
}

// oneSourceConfig installs a single query source in one band, so a test
// asserts about exactly one suggestion.
func oneSourceConfig(t *testing.T, app *App, src dataentryconfig.NextActionSource) {
	t.Helper()
	src.Band = "blocking"
	if src.Query == "" {
		src.Query = "type:ticket"
	}
	if src.Suggest == "" {
		src.Suggest = "{title}"
	}
	withNextActions(t, app,
		[]dataentryconfig.NextActionBand{{ID: "blocking"}},
		map[string]dataentryconfig.NextActionSource{"stalled": src})
}

// getNextActionAt is getNextAction against an explicit URL, so a test can
// pass `?world=`. The world is resolved by the real attachWorld middleware
// rather than stamped directly — that is the behavior under test.
func getNextActionAt(
	ctx context.Context, t *testing.T, app *App, path string,
) (resp nextActionResponse, status int) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody).WithContext(ctx)

	attachWorld(http.HandlerFunc(app.handleV1NextAction), app).ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	}
	return resp, rec.Code
}

// --- axis 1: source world -----------------------------------------------

// A source's declared world decides which FACE its query reads, so the
// suggestion interpolates that face's content.
func TestNextActionSourceWorld_ScopesTheQuery(t *testing.T) {
	tests := []struct {
		name        string
		sourceWorld string
		wantMessage string
		why         string
	}{
		{
			name:        "unset reads the default face",
			sourceWorld: "",
			wantMessage: "draft title",
			why:         "an absent source_world must behave exactly as before the key existed",
		},
		{
			name:        "an explicit default is the same thing",
			sourceWorld: "default",
			wantMessage: "draft title",
		},
		{
			name:        "a declared world reads that world's face",
			sourceWorld: "published",
			wantMessage: "published title",
			why:         "the source world must actually reach the store query, not just validate",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
			seedFacedTicket(t, app, "TKT-1", "draft title", "published title")
			oneSourceConfig(t, app, dataentryconfig.NextActionSource{SourceWorld: tc.sourceWorld})

			got, code := getNextAction(aliceCtx(), t, app)
			require.Equal(t, http.StatusOK, code)
			require.NotNil(t, got.Suggestion, "a candidate exists in both faces")
			require.Equal(t, tc.wantMessage, got.Suggestion.Message, tc.why)
		})
	}
}

// An entity the source's world EXCLUDES is not a candidate at all. This is the
// half that proves the scope reaches the store: under `otherwise: exclude` a
// ticket with no published face simply has nothing to match.
func TestNextActionSourceWorld_ExcludedEntityIsNotACandidate(t *testing.T) {
	app := newTestAppV1(t)
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
	// Draft only — no published face.
	seedFacedTicket(t, app, "TKT-UNPUB", "never published", "")
	oneSourceConfig(t, app, dataentryconfig.NextActionSource{SourceWorld: "published"})

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, got.Suggestion,
		"an entity with no face in the source's world must not be suggested")
}

// The CALLER's `?world=` must never redirect a source's query. The request
// world is display-only; letting it through would aim an operator's rule at a
// world the operator never named.
func TestNextActionSourceWorld_RequestWorldDoesNotScopeTheQuery(t *testing.T) {
	app := newTestAppV1(t)
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
	seedFacedTicket(t, app, "TKT-1", "draft title", "published title")
	// No source_world: this source reads the DEFAULT world, whatever the
	// request asks for.
	oneSourceConfig(t, app, dataentryconfig.NextActionSource{})

	got, code := getNextActionAt(aliceCtx(), t, app, "/api/v1/_next_action?world=published")
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
	require.Equal(t, "draft title", got.Suggestion.Message,
		"the request's world governs DISPLAY only; a source with no source_world "+
			"must still read the default world")
}

// A principal with no read grant for the source's world contributes no
// candidates from it — a SKIP, not an error, because an advisory surface must
// not break the page it sits on.
func TestNextActionSourceWorld_UnreadableWorldSkipsTheSource(t *testing.T) {
	tests := []struct {
		name    string
		permit  map[string]bool
		wantSug bool
	}{
		{
			name:    "granted world yields the suggestion",
			permit:  map[string]bool{"published": true},
			wantSug: true,
		},
		{
			name:    "denied world yields nothing",
			permit:  map[string]bool{"published": false},
			wantSug: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			// resolveDefault: the world resolves every ticket to its default
			// state, so a ticket is visible in it. Without that, "the grant
			// denied me" and "the world excluded everything" both look empty
			// and this test would pass with the grant check removed.
			app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}, resolveDefault: true})
			seedFacedTicket(t, app, "TKT-1", "draft title", "")
			oneSourceConfig(t, app, dataentryconfig.NextActionSource{SourceWorld: "published"})

			ctx := withReadGate(aliceCtx(), worldGate{permit: tc.permit})
			got, code := getNextAction(ctx, t, app)
			require.Equal(t, http.StatusOK, code,
				"a denied world is a quiet skip, never an error status")
			if tc.wantSug {
				require.NotNil(t, got.Suggestion)
			} else {
				require.Nil(t, got.Suggestion)
			}
		})
	}
}

// An infrastructure failure from the grant check is NOT a denial: rendering an
// outage as a quiet suggestion box hides it with no operator signal.
func TestNextActionSourceWorld_GateFailureIsNotADenial(t *testing.T) {
	app := newTestAppV1(t)
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}, resolveDefault: true})
	seedFacedTicket(t, app, "TKT-1", "draft title", "")
	oneSourceConfig(t, app, dataentryconfig.NextActionSource{SourceWorld: "published"})

	ctx := withReadGate(aliceCtx(), worldGate{fail: true})
	_, code := getNextAction(ctx, t, app)
	require.NotEqual(t, http.StatusOK, code,
		"a gate outage must surface, not degrade to a silently empty box")
}

// --- axis 2: visible worlds ---------------------------------------------

// The allow list gates DISPLAY against the world the reader is browsing.
func TestNextActionVisibleWorlds_GatesDisplay(t *testing.T) {
	tests := []struct {
		name    string
		visible []string
		path    string
		wantSug bool
		why     string
	}{
		{
			name:    "unset shows in the default world",
			visible: nil,
			path:    "/api/v1/_next_action",
			wantSug: true,
		},
		{
			name:    "unset shows in a non-default world too",
			visible: nil,
			path:    "/api/v1/_next_action?world=published",
			wantSug: true,
			why:     "an unset allow list matches ALL worlds — that is the documented default",
		},
		{
			name:    "listed world shows it",
			visible: []string{"published"},
			path:    "/api/v1/_next_action?world=published",
			wantSug: true,
		},
		{
			name:    "unlisted world hides it",
			visible: []string{"published"},
			path:    "/api/v1/_next_action",
			wantSug: false,
			why:     "the reader is in the default world, which the allow list omits",
		},
		{
			name:    "an explicit default entry shows in the default world",
			visible: []string{"default"},
			path:    "/api/v1/_next_action",
			wantSug: true,
		},
		{
			name:    "an explicit default entry hides elsewhere",
			visible: []string{"default"},
			path:    "/api/v1/_next_action?world=published",
			wantSug: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
			seedFacedTicket(t, app, "TKT-1", "draft title", "published title")
			// No source_world: this axis must work independently of the other.
			oneSourceConfig(t, app, dataentryconfig.NextActionSource{VisibleWorlds: tc.visible})

			got, code := getNextActionAt(aliceCtx(), t, app, tc.path)
			require.Equal(t, http.StatusOK, code)
			if tc.wantSug {
				require.NotNil(t, got.Suggestion, tc.why)
			} else {
				require.Nil(t, got.Suggestion, tc.why)
			}
		})
	}
}

// --- the two axes are orthogonal ----------------------------------------

// The headline case from the design: a source QUERIES the world where
// unfinished work lives, while remaining VISIBLE in the world people browse.
// Collapsing the two keys into one would make this unexpressible.
func TestNextActionWorlds_AxesAreIndependent(t *testing.T) {
	app := newTestAppV1(t)
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
	seedFacedTicket(t, app, "TKT-1", "draft title", "published title")
	oneSourceConfig(t, app, dataentryconfig.NextActionSource{
		// Read the published face...
		SourceWorld: "published",
		// ...but only nag about it while the reader is in the DEFAULT world.
		VisibleWorlds: []string{"default"},
	})

	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion, "visible in the default world")
	require.Equal(t, "published title", got.Suggestion.Message,
		"the SOURCE world decided which face was read, independently of where it is shown")

	// Same source, reader now in `published`: the query world is unchanged,
	// but display is not permitted there.
	got, code = getNextActionAt(aliceCtx(), t, app, "/api/v1/_next_action?world=published")
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, got.Suggestion, "not visible in the published world")
}

// --- the route ----------------------------------------------------------

// `?world=` is ACCEPTED on this route now (it used to 422 world_unsupported),
// because it selects the display world rather than scoping a read.
func TestNextActionRoute_AcceptsWorldParam(t *testing.T) {
	require.True(t, worldCapablePath("/api/v1/_next_action"),
		"the next-action route must admit ?world= to gate display per world")
}

// A WRITE still refuses the parameter, via the shared read-only rule in
// attachWorld. Feedback addresses a source and an entity id directly.
func TestNextActionRoute_FeedbackRefusesWorldParam(t *testing.T) {
	app := newTestAppV1(t)
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/_next_action?world=published", http.NoBody).WithContext(aliceCtx())
	attachWorld(http.HandlerFunc(app.handleV1NextAction), app).ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Contains(t, rec.Body.String(), "world_read_only")
}
