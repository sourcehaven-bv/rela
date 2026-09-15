package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// scopeSchema declares the archived story end to end: a default that hides
// archived rows, plus a named scope that shows only them. The default uses
// `~=`, which does NOT lower to a store predicate, so the Go-side filter is
// exercised rather than only the pushdown.
const scopeSchema = `version: "1.0"
namespace: https://example.org/test#
entities:
  taak:
    label: Taak
    plural: taken
    id_prefix: TAAK
    properties:
      title: {type: string, required: true}
      status: {type: string}
    query_scopes:
      default: "entity.status ~= 'gearchiveerd'"
      archief: "entity.status == 'gearchiveerd'"
`

// newScopeTestApp builds an app over scopeSchema with the REAL scope
// compiler wired, so this exercises schema -> compile -> resolve -> filter
// rather than a stubbed evaluator.
func newScopeTestApp(t *testing.T) *App {
	t.Helper()
	meta, err := metamodel.Parse([]byte(scopeSchema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	cfg := &dataentryconfig.Config{
		Forms:   make(map[string]dataentryconfig.Form),
		Lists:   make(map[string]dataentryconfig.List),
		Views:   make(map[string]dataentryconfig.ViewConfig),
		Kanbans: make(map[string]dataentryconfig.Kanban),
	}
	app := newAppFromParts(cfg, meta, newFixture())
	if err := app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)); err != nil {
		t.Fatalf("wire query scopes: %v", err)
	}
	for _, e := range []*entity.Entity{
		{ID: "TAAK-1", Type: "taak", Properties: map[string]any{"title": "open werk", "status": "todo"}},
		{ID: "TAAK-2", Type: "taak", Properties: map[string]any{"title": "afgerond", "status": "gereed"}},
		{ID: "TAAK-3", Type: "taak", Properties: map[string]any{"title": "oud", "status": "gearchiveerd"}},
	} {
		seedEntity(app, e)
	}
	return app
}

// TestQueryScopes_EndToEnd drives the whole chain out of the HTTP list
// endpoint. The unit tests stub the evaluator; this one does not, so it is
// what proves the pieces actually fit together.
func TestQueryScopes_EndToEnd(t *testing.T) {
	app := newScopeTestApp(t)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "no parameter applies the type's default scope",
			query: "",
			want:  []string{"TAAK-1", "TAAK-2"},
		},
		{
			name:  "a named scope replaces the default",
			query: "query_scope=archief",
			want:  []string{"TAAK-3"},
		},
		{
			name:  "the implicit all scope withdraws the default",
			query: "query_scope=all",
			want:  []string{"TAAK-1", "TAAK-2", "TAAK-3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scopeListIDs(t, app, tc.query)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestQueryScopes_UnknownNameIsRefused pins AC3 at the HTTP boundary: a typo
// must fail, never fall back to unfiltered. Falling back would show every
// archived row on a screen the operator scoped, and nothing would say so.
func TestQueryScopes_UnknownNameIsRefused(t *testing.T) {
	app := newScopeTestApp(t)
	rec := scopeList(app, "query_scope=archieff")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an undeclared scope name returned %d with body %s; want 400 — a mistyped "+
			"parameter is the caller's error, and the pipeline's catch-all 500 would blame "+
			"free-text search, a subsystem the request never reached", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "archieff") {
		t.Errorf("the 400 must name the scope the caller asked for, so the typo is "+
			"visible without server logs; got %s", rec.Body)
	}
}

// TestQueryScopes_RepeatedParamIsRefused pins the param-append defence.
// Get() takes the FIRST value, so `all` followed by `archief` would read
// everything under a request that also asked for the archive.
func TestQueryScopes_RepeatedParamIsRefused(t *testing.T) {
	app := newScopeTestApp(t)
	rec := scopeList(app, "query_scope=all&query_scope=archief")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a repeated query_scope returned %d; want 400 rather than picking one", rec.Code)
	}
}

func scopeList(app *App, rawQuery string) *httptest.ResponseRecorder {
	url := "/api/v1/taken"
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "taak", "taken")
	return rec
}

func scopeListIDs(t *testing.T, app *App, rawQuery string) []string {
	t.Helper()
	rec := scopeList(app, rawQuery)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ids := make([]string, 0, len(resp.Data))
	for _, d := range resp.Data {
		ids = append(ids, d.ID)
	}
	return ids
}
