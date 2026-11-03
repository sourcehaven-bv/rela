package dataentry

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// withPlacement republishes the config with one list view over `ticket`
// ("open-tickets") and lets the test adjust it. Same copy-then-publish
// shape as withNextActions.
func withPlacement(t *testing.T, app *App, edit func(cfg *dataentryconfig.Config)) {
	t.Helper()
	s := app.State()
	cfg := *s.Cfg
	cfg.Lists = maps.Clone(cfg.Lists)
	if cfg.Lists == nil {
		cfg.Lists = map[string]dataentryconfig.List{}
	}
	cfg.Lists["open-tickets"] = dataentryconfig.List{EntityType: "ticket", Title: "Open"}
	if edit != nil {
		edit(&cfg)
	}
	app.schema.Publish(&Schema{
		Cfg: &cfg, Meta: s.Meta, StyleMap: s.StyleMap,
		StyledTypes: s.StyledTypes, OpenAPIGen: s.OpenAPIGen,
	})
}

// getNextActionOn requests a suggestion for one page. params is the query
// string, e.g. {"surface": "entity", "view": "ticket", "entity": "TKT-1"}.
func getNextActionOn(
	ctx context.Context, t *testing.T, app *App, params map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_next_action?"+q.Encode(), http.NoBody).WithContext(ctx)
	app.handleV1NextAction(rec, req)
	return rec
}

func decodeNextAction(t *testing.T, rec *httptest.ResponseRecorder) nextActionResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp nextActionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp
}

func entityPage(id string) map[string]string {
	return map[string]string{"surface": "entity", "view": "ticket", "entity": id}
}

var contextSource = map[string]dataentryconfig.NextActionSource{
	"about-this": {Band: "b", Context: "ticket", Suggest: "Look at {title}"},
}

func TestNextAction_ContextSourceResolvesTheViewedEntity(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001", "TKT-002")
	withPlacement(t, app, nil)
	withNextActions(t, app, oneBand, contextSource)

	t.Run("on the entity page it suggests about that entity", func(t *testing.T) {
		got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, entityPage("TKT-002")))
		require.NotNil(t, got.Suggestion)
		require.Equal(t, "TKT-002", got.Suggestion.EntityID)
		require.Equal(t, "Look at Ticket TKT-002", got.Suggestion.Message)
	})
	t.Run("a faced address resolves by its bare id", func(t *testing.T) {
		got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, entityPage("TKT-001@draft")))
		require.NotNil(t, got.Suggestion)
		require.Equal(t, "TKT-001", got.Suggestion.EntityID)
	})
	t.Run("on a list page it is silent", func(t *testing.T) {
		got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app,
			map[string]string{"surface": "list", "view": "open-tickets"}))
		require.Nil(t, got.Suggestion)
	})
	t.Run("on another type's entity page it is silent", func(t *testing.T) {
		seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "F"}})
		got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app,
			map[string]string{"surface": "entity", "view": "feature", "entity": "FEAT-1"}))
		require.Nil(t, got.Suggestion)
	})
	t.Run("an id of another type is no candidate", func(t *testing.T) {
		got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, entityPage("FEAT-1")))
		require.Nil(t, got.Suggestion)
	})
	t.Run("an unplaced request resolves no context source", func(t *testing.T) {
		got, code := getNextAction(aliceCtx(), t, app)
		require.Equal(t, http.StatusOK, code)
		require.Nil(t, got.Suggestion)
	})
	t.Run("the entity page names its type for refreshes", func(t *testing.T) {
		got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, entityPage("TKT-002")))
		require.Equal(t, "ticket", got.ContextType)
	})
	t.Run("a page no context source is about names no type", func(t *testing.T) {
		got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app,
			map[string]string{"surface": "entity", "view": "feature", "entity": "FEAT-1"}))
		require.Empty(t, got.ContextType)
	})
}

// A context source must not tell a caller whether an id exists. A hidden
// entity and an id that names nothing get the byte-identical answer.
func TestNextAction_ContextHiddenEntityIsNonexistent(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	withPlacement(t, app, nil)
	withNextActions(t, app, oneBand, contextSource)

	alice := decodeNextAction(t, getNextActionOn(gateCtxFor(principalCtx("alice"), t, d), t, app, entityPage("TKT-001")))
	require.NotNil(t, alice.Suggestion, "alice may read the ticket")

	bob := gateCtxFor(principalCtx("bob"), t, d)
	hidden := getNextActionOn(bob, t, app, entityPage("TKT-001"))
	missing := getNextActionOn(bob, t, app, entityPage("TKT-999"))
	require.Equal(t, http.StatusOK, hidden.Code)
	require.Equal(t, missing.Code, hidden.Code)
	require.Equal(t, missing.Body.String(), hidden.Body.String(),
		"a hidden entity must answer exactly like one that does not exist")
}

func TestNextAction_ContextConditionUsesTheRealMatcher(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	seedEntity(app, &entity.Entity{
		ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "Done one", "status": "done"},
	})
	withPlacement(t, app, nil)
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"open-only": {Band: "b", Context: "ticket", Condition: `entity.status == "open"`, Suggest: "{id}"},
	})
	require.NoError(t, app.SetNextActionMatchers(appbuild.NextActionMatchers))

	open := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, entityPage("TKT-001")))
	require.NotNil(t, open.Suggestion)
	done := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, entityPage("TKT-002")))
	require.Nil(t, done.Suggestion, "the condition refines the context candidate")
}

func TestNextAction_PlacementSwitches(t *testing.T) {
	off, on := false, true
	listPage := map[string]string{"surface": "list", "view": "open-tickets"}
	cases := []struct {
		name string
		edit func(cfg *dataentryconfig.Config)
		page map[string]string
		want bool
	}{
		{"on by default", nil, listPage, true},
		{"other is off by default", nil, map[string]string{"surface": "other"}, false},
		{"a surface switch turns the kind off", func(cfg *dataentryconfig.Config) {
			cfg.NextActionSurfaces = map[dataentryconfig.NextActionSurface]bool{"list": false}
		}, listPage, false},
		{"a surface switch turns other on", func(cfg *dataentryconfig.Config) {
			cfg.NextActionSurfaces = map[dataentryconfig.NextActionSurface]bool{"other": true}
		}, map[string]string{"surface": "other"}, true},
		{"the view's own setting beats the surface", func(cfg *dataentryconfig.Config) {
			cfg.NextActionSurfaces = map[dataentryconfig.NextActionSurface]bool{"list": false}
			l := cfg.Lists["open-tickets"]
			l.Suggestions = &on
			cfg.Lists["open-tickets"] = l
		}, listPage, true},
		{"the view can opt out alone", func(cfg *dataentryconfig.Config) {
			l := cfg.Lists["open-tickets"]
			l.Suggestions = &off
			cfg.Lists["open-tickets"] = l
		}, listPage, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			withPlacement(t, app, tc.edit)
			withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
				"hint": {Band: "b", Count: "nonexistent_type == 0", Suggest: "hint"},
			})
			got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, tc.page))
			require.Equal(t, tc.want, got.Suggestion != nil)
		})
	}
}

func TestNextAction_VisibleOnPlacesASource(t *testing.T) {
	app := newTestAppV1(t)
	withPlacement(t, app, func(cfg *dataentryconfig.Config) {
		cfg.Lists["closed-tickets"] = dataentryconfig.List{EntityType: "ticket", Title: "Closed"}
	})
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"hint": {
			Band: "b", Count: "nonexistent_type == 0", Suggest: "hint",
			VisibleOn: []string{"dashboard", "list:open-tickets"},
		},
	})

	for page, want := range map[string]bool{
		"dashboard|":            true,
		"list|open-tickets":     true,
		"list|closed-tickets":   false,
		"search|":               false,
		"entity|ticket?TKT-001": false,
	} {
		t.Run(page, func(t *testing.T) {
			params := map[string]string{}
			surface, rest, _ := strings.Cut(page, "|")
			view, id, _ := strings.Cut(rest, "?")
			params["surface"], params["view"], params["entity"] = surface, view, id
			got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, params))
			require.Equal(t, want, got.Suggestion != nil)
		})
	}
}

func TestNextAction_BadPages(t *testing.T) {
	app := newTestAppV1(t)
	withPlacement(t, app, nil)
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"hint": {Band: "b", Count: "nonexistent_type == 0", Suggest: "hint"},
	})

	rec := getNextActionOn(aliceCtx(), t, app, map[string]string{"surface": "sidebar"})
	require.Equal(t, http.StatusBadRequest, rec.Code, "an unknown surface is a client bug")

	for name, params := range map[string]map[string]string{
		"a renamed list":          {"surface": "list", "view": "gone"},
		"a list with no view":     {"surface": "list"},
		"an unknown entity type":  {"surface": "entity", "view": "nope", "entity": "X-1"},
		"a view on the dashboard": {"surface": "dashboard", "view": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, params))
			require.Nil(t, got.Suggestion, "an unknown view goes quiet rather than failing")
		})
	}
}

// A page with suggestions switched off must cost nothing: the answer comes
// before any source runs. An enabled context source costs the same at 10 and
// 50 rows, because it reads one entity, not the type.
func TestNextAction_PlacementReadBudget(t *testing.T) {
	reads := func(t *testing.T, n int, edit func(cfg *dataentryconfig.Config)) int {
		t.Helper()
		app, counting, _, ctx := newBudgetAppWith(t, n, memstore.New(), analyzeBudgetMeta())
		withPlacement(t, app, edit)
		withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
			"about-this": {Band: "b", Context: "ticket", Suggest: "{id}"},
		})
		counting.Reset()
		got := decodeNextAction(t, getNextActionOn(ctx, t, app, entityPage("TKT-0001")))
		if edit == nil {
			require.NotNil(t, got.Suggestion, "the measured path must have resolved the entity")
		}
		return counting.Reads()
	}
	disabled := func(cfg *dataentryconfig.Config) {
		cfg.NextActionSurfaces = map[dataentryconfig.NextActionSurface]bool{"entity": false}
	}
	require.Zero(t, reads(t, 10, disabled), "a disabled page must not read")
	require.Equal(t, reads(t, 10, nil), reads(t, 50, nil), "a context source reads one entity, not the type")
}

// The context candidate is redacted before the message is built, like a
// query candidate: a visible:-hidden value must not reach the suggestion.
func TestNextAction_ContextRedactsHiddenProperties(t *testing.T) {
	app := newTestAppV1(t)
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"title": false},
	}})
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "SECRET-TITLE", "status": "open"},
	})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	withPlacement(t, app, nil)
	withNextActions(t, app, oneBand, contextSource)

	got := decodeNextAction(t, getNextActionOn(gateCtxFor(principalCtx("alice"), t, d), t, app, entityPage("TKT-001")))
	require.NotNil(t, got.Suggestion)
	require.NotContains(t, got.Suggestion.Message, "SECRET-TITLE")
	require.Contains(t, got.Suggestion.Message, "{title}", "the placeholder stays: names are not secret, values are")
}

// A client that sends no surface predates placement and was a dashboard
// client, so the operator's dashboard switch applies to it.
func TestNextAction_UnplacedIsTheDashboard(t *testing.T) {
	app := newTestAppV1(t)
	withPlacement(t, app, func(cfg *dataentryconfig.Config) {
		cfg.NextActionSurfaces = map[dataentryconfig.NextActionSurface]bool{"dashboard": false}
	})
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"hint": {Band: "b", Count: "nonexistent_type == 0", Suggest: "hint"},
	})
	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, got.Suggestion)
}

func TestNextActionCurrent(t *testing.T) {
	parse := func(q string) *nextaction.Current {
		return nextActionCurrent(httptest.NewRequest(http.MethodGet, "/api/v1/_next_action?"+q, http.NoBody))
	}
	require.Nil(t, parse(""), "no source is no current")
	require.Nil(t, parse("current_entity=T-1"), "an entity without a source is no current")
	require.Equal(t, &nextaction.Current{Source: "s", EntityID: "T-1", Variant: "v"},
		parse("current_source=s&current_entity=T-1&current_variant=v"))
}

// A context source records no impressions, and "Seen it" snoozes it for the
// source's cooldown. Without the snooze, the next refresh brings it back,
// because the engine ignores impressions for context sources.
func TestNextAction_SeenHidesAContextSuggestion(t *testing.T) {
	app := newTestAppV1(t)
	seedNextActionTickets(t, app, "TKT-001")
	withPlacement(t, app, nil)
	withNextActions(t, app, oneBand, contextSource)
	page := entityPage("TKT-001")
	body := func(kind string) string {
		return `{"source":"about-this","entity_id":"TKT-001","kind":"` + kind + `"}`
	}

	require.Equal(t, http.StatusNoContent, postFeedback(aliceCtx(), t, app, body("shown")))
	got := decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, page))
	require.NotNil(t, got.Suggestion, "an impression does not hide a context suggestion")

	require.Equal(t, http.StatusNoContent, postFeedback(aliceCtx(), t, app, body("seen")))
	got = decodeNextAction(t, getNextActionOn(aliceCtx(), t, app, page))
	require.Nil(t, got.Suggestion, "seen it hides it")
}

// For a query source "seen" changes nothing: its impression already hides it.
func TestNextAction_SeenIsANoOpForAQuerySource(t *testing.T) {
	app := newTestAppV1(t)
	withNextActions(t, app, oneBand, map[string]dataentryconfig.NextActionSource{
		"hint": {Band: "b", Count: "nonexistent_type == 0", Suggest: "hint"},
	})
	require.Equal(t, http.StatusNoContent, postFeedback(aliceCtx(), t, app, `{"source":"hint","kind":"seen"}`))
	got, code := getNextAction(aliceCtx(), t, app)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.Suggestion)
}
