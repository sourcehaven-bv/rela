package dataentry

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

var brokenViewConditions = []struct {
	name, condition string
}{
	{"unknown property", "entity.no_such_property == 'x'"},
	{"unknown property in a traversal", "related(entity, 'implements', { no_such_property = 'x' })"},
}

func seedOpenAndClosedTickets(t *testing.T, app *App) {
	t.Helper()
	for _, tc := range []struct{ id, status string }{
		{"TKT-001", "open"},
		{"TKT-002", "closed"},
	} {
		require.NoError(t, app.Services().Store.CreateEntity(t.Context(), &entity.Entity{
			ID: tc.id, Type: "ticket",
			Properties: map[string]any{"title": "t", "status": tc.status},
		}))
	}
}

func getTickets(t *testing.T, app *App, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	rec := httptest.NewRecorder()
	app.entities.handleV1ListEntities(rec, req, "ticket", "tickets")
	return rec
}

// One broken view condition fails only its own view closed. The view next to
// it keeps filtering, and the broken view answers an error instead of every
// row (the fail-open this guards against).
func TestViewCondition_BrokenConditionFailsOnlyItsOwnView(t *testing.T) {
	views := []struct {
		name  string
		param string
		set   func(app *App, id, condition string)
	}{
		{"list", "list_id", func(app *App, id, condition string) {
			app.Cfg().Lists[id] = dataentryconfig.List{EntityType: "ticket", Condition: condition}
		}},
		{"kanban", "kanban_id", func(app *App, id, condition string) {
			if app.Cfg().Kanbans == nil {
				app.Cfg().Kanbans = map[string]dataentryconfig.Kanban{}
			}
			app.Cfg().Kanbans[id] = dataentryconfig.Kanban{
				EntityType: "ticket", ColumnProperty: "status", Condition: condition,
			}
		}},
	}
	for _, view := range views {
		for _, broken := range brokenViewConditions {
			t.Run(view.name+"/"+broken.name, func(t *testing.T) {
				app := newTestAppV1(t)
				view.set(app, "good", "entity.status == 'open'")
				view.set(app, "broken", broken.condition)
				wireRealViewConditions(t, app)
				seedOpenAndClosedTickets(t, app)

				require.Equal(t, []string{"TKT-001"},
					listIDs(t, app, "/api/v1/tickets?"+view.param+"=good"))

				rec := getTickets(t, app, "/api/v1/tickets?"+view.param+"=broken")
				require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), "view_condition_not_compiled")
				require.NotContains(t, rec.Body.String(), "TKT-00")

				require.Len(t, listIDs(t, app, "/api/v1/tickets"), 2,
					"a request naming no view stays unconstrained")
			})
		}
	}
}

// The nav status and page step paths read a list's rows through
// configuredListRows. A broken list condition errors there; the good list
// beside it still counts only its own rows.
func TestViewCondition_BrokenConditionFailsConfiguredListRows(t *testing.T) {
	for _, broken := range brokenViewConditions {
		t.Run(broken.name, func(t *testing.T) {
			app := newTestAppV1(t)
			app.Cfg().Lists["good"] = dataentryconfig.List{EntityType: "ticket", Condition: "entity.status == 'open'"}
			app.Cfg().Lists["broken"] = dataentryconfig.List{EntityType: "ticket", Condition: broken.condition}
			wireRealViewConditions(t, app)
			seedOpenAndClosedTickets(t, app)
			s := app.State()

			_, rows, err := configuredListRows(t.Context(), app, s, "good", s.Cfg.Lists["good"], nil, nil)
			require.NoError(t, err)
			require.Len(t, rows, 1)

			_, rows, err = configuredListRows(t.Context(), app, s, "broken", s.Cfg.Lists["broken"], nil, nil)
			require.ErrorIs(t, err, errViewConditionNotCompiled)
			require.Nil(t, rows)
		})
	}
}

// Exporting a list whose condition is broken fails rather than exporting
// every row.
func TestExport_List_BrokenConditionFails(t *testing.T) {
	requireCp(t)
	app := newExportApp(t)
	app.Cfg().Lists["tickets"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  brokenViewConditions[0].condition,
		Columns:    []dataentryconfig.ListColumn{{Property: "title"}},
	}
	require.NoError(t, app.SetViewConditions(AdaptViewConditions(appbuild.ViewConditions)))
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Finished work", "status": "closed"}})

	rec := exportList(t.Context(), app)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "Finished work")
}

// The problems are logged once when the compiler is installed, not on each
// request.
func TestViewCondition_BrokenConditionLoggedOnceAtInstall(t *testing.T) {
	logged := logtest.Capture(t, slog.LevelError)

	app := newTestAppV1(t)
	app.Cfg().Lists["broken"] = dataentryconfig.List{EntityType: "ticket", Condition: brokenViewConditions[0].condition}
	wireRealViewConditions(t, app)
	require.Equal(t, 1, strings.Count(logged(), "its view fails closed"), logged())

	getTickets(t, app, "/api/v1/tickets?list_id=broken")
	getTickets(t, app, "/api/v1/tickets?list_id=broken")
	require.Equal(t, 1, strings.Count(logged(), "its view fails closed"), logged())
	require.Contains(t, logged(), `lists[\"broken\"]`)
}
