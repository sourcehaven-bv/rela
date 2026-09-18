package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/conditionlint"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// wireRealViewConditions supplies the production compiler, so this exercises
// the whole chain — config → compile → lookup → evaluate → HTTP response —
// rather than a stub that could agree with a broken implementation.
//
// It mirrors what cmd/rela-server does. conditionlint is importable from a
// TEST in this package (arch-lint constrains the package's own imports, and
// the cycle that forces appbuild's restated interface does not exist here).
func wireRealViewConditions(t *testing.T, app *App) {
	t.Helper()
	require.NoError(t, app.SetViewConditions(
		AdaptViewConditions(conditionlint.ViewConditionMatchers)))
}

func listIDs(t *testing.T, app *App, url string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp v1.ListResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	ids := make([]string, 0, len(resp.Data))
	for _, e := range resp.Data {
		ids = append(ids, e.ID)
	}
	return ids
}

// The disjunctive rule this whole feature exists for, end to end: show every
// ticket that is not closed, PLUS closed ones — a membership rule a flat
// ANDed `filters:` list cannot express at all.
func TestViewCondition_DisjunctiveRuleFiltersTheList(t *testing.T) {
	app := newTestAppV1(t)
	app.Cfg().Lists["recent"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "entity.status ~= 'closed' or entity.title == 'keep me'",
	}
	wireRealViewConditions(t, app)

	for _, tc := range []struct{ id, status, title string }{
		{"TKT-001", "open", "a"},
		{"TKT-002", "closed", "b"},
		{"TKT-003", "closed", "keep me"},
		{"TKT-004", "doing", "c"},
	} {
		require.NoError(t, app.Services().Store.CreateEntity(t.Context(), &entity.Entity{
			ID: tc.id, Type: "ticket",
			Properties: map[string]any{"title": tc.title, "status": tc.status},
		}))
	}

	got := listIDs(t, app, "/api/v1/tickets?list_id=recent")
	require.ElementsMatch(t, []string{"TKT-001", "TKT-003", "TKT-004"}, got,
		"closed tickets are excluded unless the second disjunct keeps them")

	// The generic endpoint stays generic: no list_id, no condition. A
	// condition is presentation, so this is the ACL-scoped superset — never
	// more than the principal may see.
	all := listIDs(t, app, "/api/v1/tickets")
	require.Len(t, all, 4, "a request naming no view is unconstrained")
}

// The count must describe the same population as the page. Paging before the
// condition would report a total for the unfiltered set — BUG-5OAQUG's shape.
func TestViewCondition_TotalMatchesTheFilteredPopulation(t *testing.T) {
	app := newTestAppV1(t)
	app.Cfg().Lists["open_only"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "entity.status == 'open'",
	}
	wireRealViewConditions(t, app)

	for i, status := range []string{"open", "closed", "open", "closed", "open"} {
		require.NoError(t, app.Services().Store.CreateEntity(t.Context(), &entity.Entity{
			ID: string(rune('A'+i)) + "-1", Type: "ticket",
			Properties: map[string]any{"title": "t", "status": status},
		}))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?list_id=open_only&per_page=2", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp v1.ListResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Len(t, resp.Data, 2, "the page is capped by per_page")
	require.Equal(t, 3, resp.Meta.Total,
		"total counts the CONDITIONED population, not every row of the type")
}

// An unknown list id must not silently behave like a configured one. It means
// "no condition" — the same as omitting it — rather than an error, because a
// stale bookmark should degrade to the unfiltered view, not a 500.
func TestViewCondition_UnknownListIDIsUnconstrained(t *testing.T) {
	app := newTestAppV1(t)
	app.Cfg().Lists["real"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "entity.status == 'open'",
	}
	wireRealViewConditions(t, app)

	require.NoError(t, app.Services().Store.CreateEntity(t.Context(), &entity.Entity{
		ID: "TKT-900", Type: "ticket",
		Properties: map[string]any{"title": "t", "status": "closed"},
	}))

	require.Len(t, listIDs(t, app, "/api/v1/tickets?list_id=ghost"), 1)
	require.Empty(t, listIDs(t, app, "/api/v1/tickets?list_id=real"))
}
