package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// newOwnedApp builds an app where a task may be owned by a task (subtask) or
// by a project (holds), next to a plain task relation.
func newOwnedApp(t *testing.T) *App {
	t.Helper()
	title := map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task":    {Label: "Task", IDPrefix: "TASK-", Properties: title, PropertyOrder: []string{"title"}},
			"project": {Label: "Project", IDPrefix: "PRJ-", Properties: title, PropertyOrder: []string{"title"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"subtask": {Label: "subtask", From: []string{"task"}, To: []string{"task"}, Owning: true},
			"holds":   {Label: "holds", From: []string{"project"}, To: []string{"task"}, Owning: true},
			"relates": {Label: "relates", From: []string{"task"}, To: []string{"task"}},
		},
	}
	cfg := &dataentryconfig.Config{
		Forms: map[string]dataentryconfig.Form{}, Lists: map[string]dataentryconfig.List{},
		Views: map[string]dataentryconfig.ViewConfig{}, Kanbans: map[string]dataentryconfig.Kanban{},
	}
	return newAppFromParts(cfg, meta, newFixture())
}

func seedTitled(app *App, id, typ, title string) {
	seedEntity(app, &entity.Entity{ID: id, Type: typ, Properties: map[string]any{"title": title}})
}

func ownersOfList(ctx context.Context, t *testing.T, app *App, d *acl.Declarative) map[string]*v1.EntityOwner {
	t.Helper()
	resp, rec := listEntitiesAs(ctx, t, app, d, "task", "tasks", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /tasks: %d %s", rec.Code, rec.Body)
	}
	out := map[string]*v1.EntityOwner{}
	for _, e := range resp.Data {
		out[e.ID] = e.Owner
	}
	return out
}

func readAll(t *testing.T, app *App, types ...string) *acl.Declarative {
	t.Helper()
	return mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: types}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
}

// TestOwner_ListAndSearchRows pins that a list row and a search hit carry the
// entity they are shown as part of, with the owner's title.
func TestOwner_ListAndSearchRows(t *testing.T) {
	app := newOwnedApp(t)
	seedTitled(app, "TASK-1", "task", "Plan launch")
	seedTitled(app, "TASK-2", "task", "Book venue")
	seedTitled(app, "TASK-3", "task", "Unrelated")
	seedRelation(app, entity.NewRelation("TASK-1", "subtask", "TASK-2"))
	seedRelation(app, entity.NewRelation("TASK-3", "relates", "TASK-2"))
	d := readAll(t, app, "task", "project")
	app.acl = d

	owners := ownersOfList(aliceCtx(), t, app, d)
	want := &v1.EntityOwner{ID: "TASK-1", Type: "task", Title: "Plan launch", Relation: "subtask"}
	if got := owners["TASK-2"]; got == nil || *got != *want {
		t.Errorf("TASK-2 owner = %+v, want %+v", got, want)
	}
	for _, id := range []string{"TASK-1", "TASK-3"} {
		if owners[id] != nil {
			t.Errorf("%s has owner %+v, want none", id, owners[id])
		}
	}

	resp, rec := searchAs(aliceCtx(), t, app, d, "venue")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /_search: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].Owner == nil || resp.Data[0].Owner.ID != "TASK-1" {
		t.Errorf("search hit = %+v, want TASK-2 owned by TASK-1", resp.Data)
	}
}

// TestOwner_Irregular pins the readable-parents rules on data the write path
// would refuse but an import or hand edit can produce.
func TestOwner_Irregular(t *testing.T) {
	tests := []struct {
		name  string
		read  []string
		edges [][3]string
		want  map[string]string // child -> owner id; absent means none
	}{
		{
			name:  "hidden owner is no owner and does not leak",
			read:  []string{"task"},
			edges: [][3]string{{"PRJ-1", "holds", "TASK-1"}},
			want:  map[string]string{},
		},
		{
			name:  "a hidden second owner does not hide the readable one",
			read:  []string{"task"},
			edges: [][3]string{{"PRJ-1", "holds", "TASK-1"}, {"TASK-2", "subtask", "TASK-1"}},
			want:  map[string]string{"TASK-1": "TASK-2"},
		},
		{
			name:  "two readable owners is ambiguous",
			read:  []string{"task", "project"},
			edges: [][3]string{{"PRJ-1", "holds", "TASK-1"}, {"TASK-2", "subtask", "TASK-1"}},
			want:  map[string]string{},
		},
		{
			name:  "an owned parent is not an owner",
			read:  []string{"task"},
			edges: [][3]string{{"TASK-3", "subtask", "TASK-2"}, {"TASK-2", "subtask", "TASK-1"}},
			want:  map[string]string{"TASK-2": "TASK-3"},
		},
		{
			name:  "mutual ownership has no owner, so no redirect loop",
			read:  []string{"task"},
			edges: [][3]string{{"TASK-1", "subtask", "TASK-2"}, {"TASK-2", "subtask", "TASK-1"}},
			want:  map[string]string{},
		},
		{
			name:  "self ownership is no owner",
			read:  []string{"task"},
			edges: [][3]string{{"TASK-1", "subtask", "TASK-1"}},
			want:  map[string]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newOwnedApp(t)
			seedTitled(app, "TASK-1", "task", "one")
			seedTitled(app, "TASK-2", "task", "two")
			seedTitled(app, "TASK-3", "task", "three")
			seedTitled(app, "PRJ-1", "project", "secret-project-title")
			for _, e := range tc.edges {
				seedRelation(app, entity.NewRelation(e[0], e[1], e[2]))
			}
			d := readAll(t, app, tc.read...)
			app.acl = d

			got := map[string]string{}
			for id, o := range ownersOfList(aliceCtx(), t, app, d) {
				if o != nil {
					got[id] = o.ID
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("owners = %v, want %v", got, tc.want)
			}
			_, rec := listEntitiesAs(aliceCtx(), t, app, d, "task", "tasks", "")
			if !slices.Contains(tc.read, "project") {
				for _, leak := range []string{"PRJ-1", "secret-project-title"} {
					if strings.Contains(rec.Body.String(), leak) {
						t.Errorf("hidden owner %q leaked into the list body", leak)
					}
				}
			}
		})
	}
}

// TestOwner_ReadBudget pins that resolving owners costs the same number of
// reads for 10 rows as for 50 (TKT-1U8XYN): one relation query and one header
// batch per level, never one per row.
func TestOwner_ReadBudget(t *testing.T) {
	reads := func(n int) (int, int) {
		app := newOwnedApp(t)
		seedTitled(app, "TASK-P", "task", "parent")
		rows := []*entity.Entity{}
		for i := range n {
			id := fmt.Sprintf("TASK-C%d", i)
			seedTitled(app, id, "task", "child")
			seedRelation(app, entity.NewRelation("TASK-P", "subtask", id))
			rows = append(rows, &entity.Entity{ID: id, Type: "task"})
		}
		d := readAll(t, app, "task")
		app.acl = d
		counting := storetest.NewCounting(app.store)
		headerCalls := 0
		o := appOwners(app)
		o.store = counting
		inner := o.headers
		o.headers = func(ctx context.Context, ids []string) (map[string]store.EntityHeader, error) {
			headerCalls++
			return inner(ctx, ids)
		}
		got, err := o.resolve(gateCtxFor(aliceCtx(), t, d), rows)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != n {
			t.Fatalf("resolved %d owners, want %d", len(got), n)
		}
		return counting.Reads(), headerCalls
	}
	r10, h10 := reads(10)
	r50, h50 := reads(50)
	if r10 != r50 || h10 != h50 {
		t.Errorf("reads grow with rows: 10 rows = %d store reads + %d header batches, 50 rows = %d + %d",
			r10, h10, r50, h50)
	}
}

// TestOwner_ViewCarriesOwnerAndRelatedSection pins the detail view's half: an
// owned entity's view names its owner, and the owner's `display: related`
// section lists the owned entity with its fields, its owner and no body.
func TestOwner_ViewCarriesOwnerAndRelatedSection(t *testing.T) {
	app := newOwnedApp(t)
	app.Cfg().Views["task"] = ViewConfig{
		Entry:    ViewEntry{Type: "task"},
		Traverse: []ViewTraverse{{From: "entry", Follow: "subtask", CollectAs: "subtasks"}},
		Sections: []ViewSection{{
			Heading: "Subtasks", Source: "subtasks", Display: dataentryconfig.DisplayRelated,
			Fields: []dataentryconfig.ViewSectionField{{Property: "title"}},
		}},
	}
	seedTitled(app, "TASK-1", "task", "Plan launch")
	seedEntity(app, &entity.Entity{
		ID: "TASK-2", Type: "task", Properties: map[string]any{"title": "Book venue"}, Content: "body text",
	})
	seedRelation(app, entity.NewRelation("TASK-1", "subtask", "TASK-2"))
	d := readAll(t, app, "task")
	app.acl = d

	decode := func(id string) v1.ViewResponse {
		t.Helper()
		rec := viewsAs(aliceCtx(), t, app, d, "task", id)
		if rec.Code != http.StatusOK {
			t.Fatalf("_views/%s: %d %s", id, rec.Code, rec.Body)
		}
		var resp v1.ViewResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}

	child := decode("TASK-2")
	if child.Entry.Owner == nil || child.Entry.Owner.ID != "TASK-1" || child.Entry.Owner.Title != "Plan launch" {
		t.Errorf("TASK-2 owner = %+v, want TASK-1 (Plan launch)", child.Entry.Owner)
	}

	parent := decode("TASK-1")
	if parent.Entry.Owner != nil {
		t.Errorf("TASK-1 owner = %+v, want none", parent.Entry.Owner)
	}
	if len(parent.Sections) != 1 || len(parent.Sections[0].Entities) != 1 {
		t.Fatalf("sections = %+v, want one related section with one entity", parent.Sections)
	}
	got := parent.Sections[0].Entities[0]
	if got.ID != "TASK-2" || len(got.Fields) != 1 || got.Content != "" {
		t.Errorf("related row = %+v, want TASK-2 with one field and no body", got)
	}
	if got.Owner == nil || got.Owner.ID != "TASK-1" {
		t.Errorf("related row owner = %+v, want TASK-1", got.Owner)
	}
}

// The type-allowlist fallback writes the store directly, so it applies the
// owning rules itself. `holds` is project -> task; a task as the source is a
// soft condition that takes the fallback.
func TestOwner_FallbackWriteAppliesOwningRules(t *testing.T) {
	app := newOwnedApp(t)
	app.broker = newEventBroker()
	bindRepo(app, t.TempDir())
	seedTitled(app, "PRJ-1", "project", "Launch")
	seedTitled(app, "TASK-1", "task", "Source")
	seedTitled(app, "TASK-2", "task", "Owned")
	seedTitled(app, "TASK-3", "task", "Free")
	seedRelation(app, &entity.Relation{From: "PRJ-1", Type: "holds", To: "TASK-2"})

	patch := func(target string) *httptest.ResponseRecorder {
		body := `{"relations":{"holds":{"data":[{"type":"task","id":"` + target + `"}]}}}`
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/tasks/TASK-1", strings.NewReader(body))
		rec := httptest.NewRecorder()
		app.write.handleV1UpdateEntity(rec, req, "task", "tasks", "TASK-1")
		return rec
	}

	if rec := patch("TASK-2"); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "owning_rule") {

		t.Fatalf("second owner: got %d %s, want 422 owning_rule", rec.Code, rec.Body)
	}
	if _, err := app.store.GetRelation(t.Context(),
		entity.RelationKey{From: "TASK-1", Type: "holds", To: "TASK-2"}); err == nil {
		t.Fatal("refused edge was written")
	}

	if rec := patch("TASK-3"); rec.Code != http.StatusOK {
		t.Fatalf("free target: got %d %s, want 200", rec.Code, rec.Body)
	}
	if _, err := app.store.GetRelation(t.Context(),
		entity.RelationKey{From: "TASK-1", Type: "holds", To: "TASK-3"}); err != nil {
		t.Fatalf("edge to a free target was not written: %v", err)
	}
}
