package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// A list scope's list_id (TKT-M07C97Y) applies the list's condition, so a
// walk over the list counts and links exactly the rows the list shows.

func positionFor(t *testing.T, app *App, id string, scope ScopeDescriptor) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(scope)
	require.NoError(t, err)
	q := url.Values{"id": {id}, "scope": {string(raw)}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_position?"+q.Encode(), http.NoBody).
		WithContext(principalCtx("alice"))
	rec := httptest.NewRecorder()
	app.handleV1EntityPosition(rec, req)
	return rec
}

func TestPosition_ListIDAppliesCondition(t *testing.T) {
	app := newTestAppV1(t)
	wireRealViewConditions(t, app)
	for _, tc := range []struct{ id, status string }{{"TKT-1", "done"}, {"TKT-2", "open"}, {"TKT-3", "done"}} {
		seedEntity(app, &entity.Entity{ID: tc.id, Type: "ticket", Properties: map[string]any{"title": tc.id, "status": tc.status}})
	}
	installPagesConfig(app, map[string]dataentryconfig.List{
		"done": {EntityType: "ticket", Condition: "entity.status == 'done'"},
	}, nil, nil)

	scope := ScopeDescriptor{Source: "list", Type: "ticket", Sort: "title"}
	rec := positionFor(t, app, "TKT-3", scope)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var pos v1.Position
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pos))
	require.Equal(t, 3, pos.Total, "without list_id the condition does not apply")

	scope.ListID = "done"
	rec = positionFor(t, app, "TKT-3", scope)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pos))
	require.Equal(t, v1.Position{
		Prev: &v1.PositionRef{ID: "TKT-1", Type: "ticket", Address: "TKT-1"}, Current: 2, Total: 2,
	}, pos)

	rec = positionFor(t, app, "TKT-2", scope)
	require.Equal(t, http.StatusNotFound, rec.Code, "a row the condition drops is not in the scope")
}

func TestScopeFromParam_ListIDNeedsSourceList(t *testing.T) {
	meta := newTestAppV1(t).Meta()
	for _, raw := range []string{
		`{"source":"search","q":"x","list_id":"all"}`,
		`{"source":"pile","pile":"p","list_id":"all"}`,
	} {
		_, ok, reason := scopeFromParam(raw, meta)
		require.False(t, ok, raw)
		require.NotEmpty(t, reason)
	}
	d, ok, _ := scopeFromParam(`{"source":"list","type":"ticket","list_id":"all"}`, meta)
	require.True(t, ok)
	require.Equal(t, "all", d.toQuery().Get(listIDParam))
}
