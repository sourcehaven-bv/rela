package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// runListFilterAs is runListFilter for a request made as user.
func runListFilterAs(t *testing.T, app *App, user, query string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?"+query, http.NoBody)
	req = req.WithContext(principal.With(req.Context(), principal.Principal{User: user, Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp v1.ListResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	ids := make([]string, len(resp.Data))
	for i, e := range resp.Data {
		ids[i] = e.ID
	}
	slices.Sort(ids)
	return ids
}

// TestV1FilteringUserToken pins TKT-YJSRBZ: a `$user` filter value is the
// caller's principal id, on the pushdown path and on the in-memory path (`q`
// keeps a query off the pushdown), and per member of an `in` list. Without an
// identified caller it stays literal, so it does not match the row whose
// property is empty.
func TestV1FilteringUserToken(t *testing.T) {
	app := newTestAppV1(t)
	for id, status := range map[string]string{"TKT-a": "alice", "TKT-b": "bob", "TKT-e": ""} {
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket",
			Properties: map[string]any{"title": "work " + id, "status": status}})
	}
	for _, tc := range []struct {
		name, user, query string
		want              []string
	}{
		{"alice", "alice", "filter[status][eq]=$user", []string{"TKT-a"}},
		{"bob", "bob", "filter[status][eq]=$user", []string{"TKT-b"}},
		{"alice, with q (not pushed down)", "alice", "q=work&filter[status][eq]=$user", []string{"TKT-a"}},
		{"in list", "alice", "filter[status][in]=$user,bob", []string{"TKT-a", "TKT-b"}},
		{"no identified caller", principal.Unknown, "filter[status][eq]=$user", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, runListFilterAs(t, app, tc.user, tc.query))
		})
	}
	// ne also leaves out the row without a status, as it does for a literal.
	assert.Equal(t, []string{"TKT-b"}, runListFilterAs(t, app, "alice", "filter[status][ne]=$user"))
}
