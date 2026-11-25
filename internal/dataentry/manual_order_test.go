package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/listorder"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Manual list order (TKT-AVNEX5T).

type manualEnv struct {
	app *App
	d   *acl.Declarative
}

func newManualEnv(t *testing.T, policy *acl.Policy, ids ...string) *manualEnv {
	t.Helper()
	cfg := &Config{App: AppConfig{Name: "Test"}, Lists: map[string]dataentryconfig.List{
		"todo": {EntityType: "ticket", Title: "Todo", Order: dataentryconfig.ListOrderManual},
		"grouped": {EntityType: "ticket", Title: "Grouped", Order: dataentryconfig.ListOrderManual,
			GroupBy: &dataentryconfig.ListGroupBy{Property: "owner"}},
		"plain": {EntityType: "ticket", Title: "Plain"},
	}}
	app := newAppFromParts(cfg, busyRowsMeta(t), newFixture())
	for i, id := range ids {
		owner := "alice"
		if i%2 == 1 {
			owner = "bob"
		}
		seedEntity(app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id, "owner": owner}})
	}
	app.setWorlds(pageWorld{})
	d := mustNewACL(t, policy, app.store)
	setTestACL(t, app, d)
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/root")
	require.NoError(t, err)
	svc, err := listorder.New(state.NewFSKV(rfs))
	require.NoError(t, err)
	require.NoError(t, SetListOrder(app, svc))
	return &manualEnv{app: app, d: d}
}

func readerPolicy() *acl.Policy {
	return &acl.Policy{
		Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"ticket", "person"}}},
		Assignments: map[string]string{"alice": "reader", "bob": "reader"},
	}
}

func (e *manualEnv) ids(t *testing.T, user, query string) ([]string, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets?"+query, http.NoBody).
		WithContext(gateCtxFor(principalCtx(user), t, e.d))
	attachWorld(http.HandlerFunc(e.app.handleV1DynamicRoutes), e.app).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp v1.ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	out := make([]string, len(resp.Data))
	for i, r := range resp.Data {
		out[i] = r.ID
	}
	return out, resp.Meta.ManualOrder
}

func (e *manualEnv) move(t *testing.T, query, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, listOrderPath+"?"+query, strings.NewReader(body)).
		WithContext(gateCtxFor(principalCtx("alice"), t, e.d))
	attachWorld(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleV1ListOrder(e.app, w, r)
	}), e.app).ServeHTTP(rec, req)
	return rec
}

const todoQuery = "list_id=todo&manual_order=1&sort=title"

func TestManualOrder_MoveThenList(t *testing.T) {
	e := newManualEnv(t, readerPolicy(), "TKT-1", "TKT-2", "TKT-3", "TKT-4")
	rec := e.move(t, todoQuery, `{"id":"TKT-3","before":"TKT-1"}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	got, manual := e.ids(t, "alice", todoQuery)
	require.True(t, manual)
	require.Equal(t, []string{"TKT-3", "TKT-1", "TKT-2", "TKT-4"}, got)

	// Another user keeps the sort.
	got, _ = e.ids(t, "bob", todoQuery)
	require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4"}, got)

	// Without manual_order the reader's own sort wins.
	got, manual = e.ids(t, "alice", "list_id=todo&sort=title")
	require.False(t, manual)
	require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4"}, got)

	// A list without order: manual ignores the parameter.
	got, manual = e.ids(t, "alice", "list_id=plain&manual_order=1&sort=title")
	require.False(t, manual)
	require.Equal(t, []string{"TKT-1", "TKT-2", "TKT-3", "TKT-4"}, got)
}

func TestManualOrder_MoveErrors(t *testing.T) {
	e := newManualEnv(t, readerPolicy(), "TKT-1", "TKT-2")
	tests := []struct {
		name, query, body string
		want              int
	}{
		{"not a manual list", "list_id=plain&manual_order=1", `{"id":"TKT-1","before":"TKT-2"}`, http.StatusNotFound},
		{"missing row", todoQuery, `{"id":"TKT-9","before":"TKT-2"}`, http.StatusNotFound},
		{"two neighbors", todoQuery, `{"id":"TKT-1","before":"TKT-2","after":"TKT-2"}`, http.StatusBadRequest},
		{"bad json", todoQuery, `{`, http.StatusBadRequest},
		{"too large", todoQuery, `{"id":"` + strings.Repeat("x", maxListOrderBody) + `"}`,
			http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := e.move(t, tc.query, tc.body)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
		})
	}
}

// A row the caller cannot read answers like a missing one, and takes no
// slot in the order.
func TestManualOrder_HiddenRow(t *testing.T) {
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"member":  {Read: []string{"person"}},
			"officer": {Read: []string{"ticket"}},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"officer-of": {Confers: "officer"}},
		Assignments:   map[string]string{"alice": "member"},
	}
	e := newManualEnv(t, policy, "TKT-1", "TKT-2", "TKT-3")
	seedEntity(e.app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "alice"}})
	seedRelation(e.app, entity.NewRelation("alice", "officer-of", "TKT-1"))
	seedRelation(e.app, entity.NewRelation("alice", "officer-of", "TKT-3"))

	hidden := e.move(t, todoQuery, `{"id":"TKT-2","before":"TKT-1"}`)
	missing := e.move(t, todoQuery, `{"id":"TKT-9","before":"TKT-1"}`)
	require.Equal(t, http.StatusNotFound, hidden.Code)
	require.Equal(t, missing.Body.String(), hidden.Body.String())

	require.Equal(t, http.StatusNoContent, e.move(t, todoQuery, `{"id":"TKT-3","before":"TKT-1"}`).Code)
	got, _ := e.ids(t, "alice", todoQuery)
	require.Equal(t, []string{"TKT-3", "TKT-1"}, got)
}

// A move made while a filter hides rows keeps those rows' places.
func TestManualOrder_FilterKeepsHiddenPlaces(t *testing.T) {
	e := newManualEnv(t, readerPolicy(), "TKT-1", "TKT-2", "TKT-3", "TKT-4")
	require.Equal(t, http.StatusNoContent, e.move(t, todoQuery, `{"id":"TKT-4","before":"TKT-1"}`).Code)
	filtered := todoQuery + "&filter[owner][eq]=alice"
	require.Equal(t, http.StatusNoContent, e.move(t, filtered, `{"id":"TKT-3","before":"TKT-1"}`).Code)
	got, _ := e.ids(t, "alice", todoQuery)
	require.Equal(t, []string{"TKT-4", "TKT-3", "TKT-1", "TKT-2"}, got)
}

// On a grouped list the order applies inside each section; sections keep
// the group sort.
func TestManualOrder_Grouped(t *testing.T) {
	e := newManualEnv(t, readerPolicy(), "TKT-1", "TKT-2", "TKT-3", "TKT-4")
	q := "list_id=grouped&manual_order=1&sort=owner,title"
	require.Equal(t, http.StatusNoContent, e.move(t, q, `{"id":"TKT-4","before":"TKT-2"}`).Code)
	got, _ := e.ids(t, "alice", q)
	require.Equal(t, []string{"TKT-1", "TKT-3", "TKT-4", "TKT-2"}, got)
}
