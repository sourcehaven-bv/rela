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
	"github.com/Sourcehaven-BV/rela/internal/principal"
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
      toegewezen_aan: {type: string}
    query_scopes:
      default: "entity.status ~= 'gearchiveerd'"
      archief: "entity.status == 'gearchiveerd'"
      mijn: "is_current_user(entity.toegewezen_aan)"
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
		{ID: "TAAK-1", Type: "taak", Properties: map[string]any{
			"title": "open werk", "status": "todo", "toegewezen_aan": "alice"}},
		{ID: "TAAK-2", Type: "taak", Properties: map[string]any{
			"title": "afgerond", "status": "gereed", "toegewezen_aan": "bob"}},
		{ID: "TAAK-3", Type: "taak", Properties: map[string]any{
			"title": "oud", "status": "gearchiveerd", "toegewezen_aan": "alice"}},
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
	return idsFromListBody(t, rec)
}

func idsFromListBody(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
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

// TestQueryScopes_IdentityScope is the ticket's headline case: a scope reading
// current_user, which is one of the commonest membership rules there is
// ("assigned to me").
//
// It needs its own test because the identity arrives by a different route than
// every other scope input. A plain scope reads properties off the row the
// store just returned; an identity scope needs a principal RESOLVED and
// stamped on ctx before any row is evaluated. Nothing about compiling or
// applying the scope reveals whether that stamping happens, so without this
// test the feature's most useful shape can be — and was — entirely unwired
// while every other test passed.
//
// The failure mode if it regresses is an error on every page of the type, not
// wrong rows. That is the correct direction (a personal list showing a
// stranger's rows is the worst version of getting this wrong) but it is still
// a broken feature, so this test asserts the rows rather than the error.
func TestQueryScopes_IdentityScope(t *testing.T) {
	app := newScopeTestApp(t)

	rec := scopeListAs(app, "alice", "query_scope=mijn")
	if rec.Code != http.StatusOK {
		t.Fatalf("an identity scope returned %d: %s\n"+
			"A scope reading current_user needs the principal stamped on ctx before "+
			"rows are evaluated; without it every page of this type errors.",
			rec.Code, rec.Body)
	}
	got := idsFromListBody(t, rec)

	// TAAK-3 is alice's too, but archived — this asserts the named scope
	// REPLACES the default rather than composing with it.
	want := []string{"TAAK-1", "TAAK-3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("alice's rows = %v, want %v", got, want)
	}

	// The discriminating half: the same request as someone else must not
	// return alice's rows.
	rec = scopeListAs(app, "bob", "query_scope=mijn")
	if rec.Code != http.StatusOK {
		t.Fatalf("identity scope for bob: %d %s", rec.Code, rec.Body)
	}
	if got := idsFromListBody(t, rec); strings.Join(got, ",") != "TAAK-2" {
		t.Errorf("bob's rows = %v, want [TAAK-2] — an identity scope that ignores "+
			"who is asking is worse than one that errors", got)
	}
}

func scopeListAs(app *App, user, rawQuery string) *httptest.ResponseRecorder {
	url := "/api/v1/taken"
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: user, Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "taak", "taken")
	return rec
}
