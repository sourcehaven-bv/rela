package dataentry

import (
	"context"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Body task counts (TKT-SQ8O0EQ) on the data-entry list surface.

const bodyTasksWireMeta = `version: "1.0"
entities:
  todo:
    label: Todo
    id_prefix: "TODO-"
    properties:
      title: {type: string}
      tasks_done: {type: integer, rollup: {body_tasks: done}}
      tasks_total: {type: integer, rollup: {body_tasks: total}}
  chore:
    label: Chore
    id_prefix: "CHO-"
    properties:
      title: {type: string}
      tasks_total: {type: integer, rollup: {body_tasks: total}}
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      title: {type: string}
`

func newBodyTasksApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(bodyTasksWireMeta))
	if err != nil {
		t.Fatal(err)
	}
	base := memstore.New()
	ctx := context.Background()
	for _, p := range []string{"PERS-A", "PERS-D"} {
		e := entity.New(p, "person")
		e.Properties["title"] = p
		if err = base.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	e := entity.New("TODO-1", "todo")
	e.Properties["title"] = "Groceries"
	e.Properties["tasks_done"] = 99 // a stored value under the name is never served
	e.Content = "- [x] milk\n- [ ] eggs\n  - [X] free range\n```\n- [ ] not a task\n```\n"
	if err = base.CreateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		App: dataentryconfig.AppConfig{Name: "todos"},
		Lists: map[string]dataentryconfig.List{
			"todos": {EntityType: "todo", Columns: []dataentryconfig.ListColumn{
				{Property: "title"}, {Property: "tasks_done"}, {Property: "tasks_total"},
			}},
		},
		Views: map[string]dataentryconfig.ViewConfig{}, Forms: map[string]dataentryconfig.Form{},
		Kanbans: map[string]dataentryconfig.Kanban{}, Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture(), appbuildtest.WithStore(base))
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all": {Read: []string{"*"}},
			"doneblind": {
				Read:    []string{"*"},
				Visible: map[string][]acl.FieldGrant{"todo": {{Field: "title"}, {Field: "tasks_total"}}},
			},
		},
		Assignments: map[string]string{"PERS-A": "all", "PERS-D": "doneblind"},
	}, app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", meta, app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	return app, d
}

// TestBodyTasks_ListColumn pins that a list serves the counts of each row's
// body, that a `visible:` grant hiding the property withholds it, and that
// filter and sort on a body count are refused.
func TestBodyTasks_ListColumn(t *testing.T) {
	app, d := newBodyTasksApp(t)
	cases := []struct {
		user        string
		done, total any
	}{
		{"PERS-A", float64(2), float64(3)},
		{"PERS-D", nil, float64(3)},
	}
	for _, tc := range cases {
		t.Run(tc.user, func(t *testing.T) {
			resp, rec := listEntitiesAs(principalCtx(tc.user), t, app, d, "todo", "todos", "list_id=todos")
			if rec.Code != http.StatusOK || len(resp.Data) != 1 {
				t.Fatalf("list = %d %s", rec.Code, rec.Body)
			}
			props := resp.Data[0].Properties
			if got, has := props["tasks_done"]; got != tc.done || (tc.done == nil && has) {
				t.Errorf("tasks_done = %v, want %v", got, tc.done)
			}
			if got := props["tasks_total"]; got != tc.total {
				t.Errorf("tasks_total = %v, want %v", got, tc.total)
			}
		})
	}
	for _, q := range []string{"filter[tasks_total][gte]=1", "sort=tasks_done"} {
		_, rec := listEntitiesAs(principalCtx("PERS-A"), t, app, d, "todo", "todos", q)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("list ?%s = %d, want 400 (%s)", q, rec.Code, rec.Body)
		}
	}
	// The refusal does not depend on the data: a type without rows refuses too.
	_, rec := listEntitiesAs(principalCtx("PERS-A"), t, app, d, "chore", "chores", "sort=tasks_total")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty list ?sort=tasks_total = %d, want 400 (%s)", rec.Code, rec.Body)
	}
}
