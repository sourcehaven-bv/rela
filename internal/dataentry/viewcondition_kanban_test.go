package dataentry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// A board's `condition:` goes through the list's path: the board names itself
// with kanban_id and the server keeps only the matching cards.
func TestViewCondition_KanbanFiltersTheBoard(t *testing.T) {
	app := newTestAppV1(t)
	if app.Cfg().Kanbans == nil {
		app.Cfg().Kanbans = map[string]dataentryconfig.Kanban{}
	}
	app.Cfg().Kanbans["today"] = dataentryconfig.Kanban{
		EntityType:     "ticket",
		ColumnProperty: "status",
		Condition:      "entity.status ~= 'closed' or entity.title == 'keep me'",
	}
	wireRealViewConditions(t, app)

	for _, tc := range []struct{ id, status, title string }{
		{"TKT-001", "open", "a"},
		{"TKT-002", "closed", "b"},
		{"TKT-003", "closed", "keep me"},
	} {
		require.NoError(t, app.Services().Store.CreateEntity(t.Context(), &entity.Entity{
			ID: tc.id, Type: "ticket",
			Properties: map[string]any{"title": tc.title, "status": tc.status},
		}))
	}

	require.ElementsMatch(t, []string{"TKT-001", "TKT-003"},
		listIDs(t, app, "/api/v1/tickets?kanban_id=today"))
	// A list id does not reach a kanban's condition.
	require.Len(t, listIDs(t, app, "/api/v1/tickets?list_id=today"), 3)
}

// A read that names both a list and a kanban is refused: taking either one
// would show the caller a view it did not ask for.
func TestViewCondition_ListAndKanbanIDIsBadRequest(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?list_id=a&kanban_id=b", http.NoBody)
	rec := httptest.NewRecorder()
	app.entities.handleV1ListEntities(rec, req, "ticket", "tickets")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "invalid_view")
}
