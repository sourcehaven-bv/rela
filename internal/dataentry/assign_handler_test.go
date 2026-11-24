package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func assignTestApp(t *testing.T) *App {
	t.Helper()
	one := 1
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"todo": {Label: "To-do", Plural: "todos", IDPrefix: "TD-", Properties: map[string]metamodel.PropertyDef{
				"title":      {Type: "string"},
				"note":       {Type: "string"},
				"status":     {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "done"}},
				"when":       {Type: metamodel.PropertyTypeEnum, Values: []string{"today", "someday"}},
				"start_date": {Type: "date"},
				"rank":       {Type: metamodel.PropertyTypeInteger},
			}},
			"project": {Label: "Project", Plural: "projects", IDPrefix: "PRJ-", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
			}},
		},
		Relations: map[string]metamodel.RelationDef{
			"in_project": {From: []string{"todo"}, To: []string{"project"}, MaxOutgoing: &one},
		},
	}
	cfg := &Config{
		Lists: map[string]dataentryconfig.List{
			"today": {EntityType: "todo", Title: "Today",
				Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}},
				Assign: &dataentryconfig.Assign{Properties: map[string]dataentryconfig.AssignValue{
					"when": {Literal: "today"}, "start_date": {Expr: "today()"}, "rank": {Literal: "3"},
				}}},
		},
		Pages: map[string]dataentryconfig.Page{
			"project": {PageHeader: dataentryconfig.PageHeader{Label: "Project", EntityType: "project"},
				Assign: &dataentryconfig.Assign{EntityType: "todo",
					Relations: map[string]string{"in_project": dataentryconfig.AssignTarget}}},
		},
	}
	f := newFixture()
	f.AddNode(&entity.Entity{ID: "TD-1", Type: "todo", Properties: map[string]any{
		"title": "Open one", "note": "keep me", "status": "open"}})
	f.AddNode(&entity.Entity{ID: "TD-2", Type: "todo", Properties: map[string]any{"title": "Done one", "status": "done"}})
	f.AddNode(&entity.Entity{ID: "PRJ-1", Type: "project", Properties: map[string]any{"title": "P"}})
	f.AddNode(&entity.Entity{ID: "PRJ-2", Type: "project", Properties: map[string]any{"title": "Q"}})
	app := newAppFromParts(cfg, meta, f)
	if err := SetAssignExprs(app, appbuild.AssignExpr); err != nil {
		t.Fatal(err)
	}
	return app
}

func postAssign(ctx context.Context, t *testing.T, app *App, body string) (int, assignResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, assignPath, bytes.NewBufferString(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1Assign(rec, req)
	var resp assignResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp
}

func TestAssign_GetResolvesValues(t *testing.T) {
	app := assignTestApp(t)
	rec := doRequest(t, app, http.MethodGet, assignPath+"?list=today")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got assignValues
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Properties["when"] != "today" || got.Properties["start_date"] != time.Now().Format("2006-01-02") {
		t.Fatalf("got %+v", got.Properties)
	}
	if rec := doRequest(t, app, http.MethodGet, assignPath+"?list=nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown list: %d", rec.Code)
	}
}

func TestAssign_PostWritesAndReportsMembership(t *testing.T) {
	app := assignTestApp(t)
	// TD-1 twice: one write, one result.
	code, resp := postAssign(t.Context(), t, app, `{"list":"today","entities":["TD-1","TD-2","TD-1"]}`)
	if code != http.StatusOK || len(resp.Results) != 2 {
		t.Fatalf("status %d: %+v", code, resp)
	}
	for _, r := range resp.Results {
		if !r.OK || r.Matches == nil {
			t.Fatalf("result %+v", r)
		}
	}
	if !*resp.Results[0].Matches || *resp.Results[1].Matches {
		t.Fatalf("matches: open row should match, done row should not: %+v", resp.Results)
	}
	got := app.mustGetEntity("TD-1")
	if got.Properties["when"] != "today" || got.Properties["note"] != "keep me" || fmt.Sprint(got.Properties["rank"]) != "3" {
		t.Fatalf("TD-1 after assign: %+v", got.Properties)
	}
}

func TestErrorCodeOf(t *testing.T) {
	for body, want := range map[string]string{
		`{"type":"https://rela.dev/errors/not_found","status":404}`: "not_found",
		`{"errors":[{"code":"rule"}]}`:                              "rule",
		`{"error":"forbidden"}`:                                     "forbidden",
		`{`:                                                         "",
	} {
		if got := errorCodeOf([]byte(body)); got != want {
			t.Errorf("%s: got %q, want %q", body, got, want)
		}
	}
}

func TestAssign_PostRejectsBadBatches(t *testing.T) {
	app := assignTestApp(t)
	for _, body := range []string{`{"list":"today","entities":[]}`, `{"list":"today","page":"project","entities":["TD-1"]}`, `{`} {
		if code, _ := postAssign(t.Context(), t, app, body); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", body, code)
		}
	}
}

func TestAssign_PageTargetRelates(t *testing.T) {
	app := assignTestApp(t)
	code, resp := postAssign(t.Context(), t, app, `{"page":"project","target":"PRJ-1","entities":["TD-1"]}`)
	if code != http.StatusOK || !resp.Results[0].OK || resp.Results[0].Matches != nil {
		t.Fatalf("status %d: %+v", code, resp)
	}
	found := false
	for r, err := range app.store.ListRelations(t.Context(), store.RelationQuery{EntityID: "TD-1", Direction: store.DirectionOutgoing}) {
		if err != nil {
			t.Fatal(err)
		}
		found = found || (r.From == "TD-1" && r.Type == "in_project" && r.To == "PRJ-1")
	}
	if !found {
		t.Fatal("in_project edge not written")
	}
	// in_project allows one edge, so a second drop moves the to-do.
	if _, resp := postAssign(t.Context(), t, app, `{"page":"project","target":"PRJ-2","entities":["TD-1"]}`); !resp.Results[0].OK {
		t.Fatalf("second drop: %+v", resp)
	}
	var peers []string
	for r, err := range app.store.ListRelations(t.Context(), store.RelationQuery{EntityID: "TD-1", Direction: store.DirectionOutgoing}) {
		if err != nil {
			t.Fatal(err)
		}
		if r.Type == "in_project" {
			peers = append(peers, r.To)
		}
	}
	if len(peers) != 1 || peers[0] != "PRJ-2" {
		t.Fatalf("in_project after second drop: %v, want [PRJ-2]", peers)
	}
	if code, _ := postAssign(t.Context(), t, app, `{"page":"project","target":"PRJ-404","entities":["TD-1"]}`); code != http.StatusNotFound {
		t.Fatalf("missing target: %d", code)
	}
}

func TestAssign_ACL(t *testing.T) {
	app := assignTestApp(t)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"reader": {Read: []string{"todo"}},
			"none":   {},
		},
		Assignments: map[string]string{"bob": "reader", "eve": "none"},
	}, app.store)
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(paths.CacheDir, 0o755)
	state := app.State()
	svc := appbuildtest.New(state.Meta, appbuildtest.WithFS(fs, paths),
		appbuildtest.WithStore(app.store), appbuildtest.WithDeclarative(d))
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, d)
	app.schema.Publish(state)
	for _, tc := range []struct {
		user string
		want int
	}{{"bob", http.StatusForbidden}, {"eve", http.StatusNotFound}} {
		ctx := gateCtxFor(principal.With(context.Background(),
			principal.Principal{User: tc.user, Tool: principal.ToolDataEntry}), t, d)
		code, resp := postAssign(ctx, t, app, `{"list":"today","entities":["TD-1"]}`)
		if code != http.StatusOK || resp.Results[0].OK || resp.Results[0].Status != tc.want {
			t.Errorf("%s: status %d, %+v; want row status %d", tc.user, code, resp, tc.want)
		}
	}
	if got := app.mustGetEntity("TD-1"); got.Properties["when"] != nil {
		t.Fatalf("refused assign wrote: %+v", got.Properties)
	}
}

func (a *App) mustGetEntity(id string) *entity.Entity {
	e, found := a.reader.writePrepRow(context.Background(), entity.Ref{ID: id})
	if !found {
		panic("missing " + id)
	}
	return e
}
