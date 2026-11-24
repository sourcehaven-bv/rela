package visibility_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const bodyTasksMeta = `version: "1.0"
entities:
  todo:
    label: Todo
    id_prefix: "TODO-"
    properties:
      title: {type: string}
      tasks_done: {type: integer, rollup: {body_tasks: done}}
      tasks_total: {type: integer, rollup: {body_tasks: total}}
`

func bodyTasksSet(t *testing.T) *rollup.Set {
	t.Helper()
	meta, err := metamodel.Parse([]byte(bodyTasksMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func createTodo(t *testing.T, st store.Store, id, body string, inaccessible bool) entity.Ref {
	t.Helper()
	e := entity.New(id, "todo")
	e.Properties["title"] = id
	e.Content = body
	if inaccessible {
		e.Content = ""
		e.Inaccessible = []entity.InaccessibleField{
			{Name: entity.InaccessibleFieldContent, Reason: entity.InaccessibleReasonGitCrypt},
		}
	}
	if err := st.CreateEntity(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	return e.Ref()
}

// TestRollups_BodyTasks pins that a body_tasks roll-up follows the body:
// the count is there where the reader can read the body, and absent where
// the body is inaccessible or a `visible:` grant hides the property.
func TestRollups_BodyTasks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	set := bodyTasksSet(t)
	st := memstore.New()
	open := createTodo(t, st, "TODO-1", "- [x] a\n- [ ] b\n```\n- [x] code\n```\n  * [X] nested\n", false)
	locked := createTodo(t, st, "TODO-2", "- [x] a\n", true)
	rows := []entity.Ref{open, locked}

	cases := []struct {
		name   string
		fields visibility.FieldScoper
		want   map[string]string
	}{
		{"body readable", visibility.NopRedactor{},
			map[string]string{"TODO-1": "map[tasks_done:2 tasks_total:3]", "TODO-2": "map[]"}},
		{"property hidden", gatedFields{"tasks_done"},
			map[string]string{"TODO-1": "map[tasks_total:3]", "TODO-2": "map[]"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := visibility.NewRollups(set, st, visibility.AllowAllReadQueries{}, tc.fields)
			if err != nil {
				t.Fatal(err)
			}
			got := r.Values(ctx, "todo", rows, store.TrivialScope(), nil)
			for id, want := range tc.want {
				if fmt.Sprint(got[id]) != want {
					t.Errorf("%s = %v, want %s", id, got[id], want)
				}
			}
		})
	}
}

// TestRollups_BodyTasksCostIsScaleFree pins the collection-read budget for
// body_tasks: one content read per page, whatever the page size, and only
// the served rows are read, whatever the store size.
func TestRollups_BodyTasksCostIsScaleFree(t *testing.T) {
	t.Parallel()
	set := bodyTasksSet(t)
	page := func(t *testing.T, pad, served int) storetest.Cost {
		t.Helper()
		mem := memstore.New()
		var rows []entity.Ref
		for i := range pad {
			ref := createTodo(t, mem, fmt.Sprintf("TODO-%04d", i), "- [x] a\n- [ ] b\n", false)
			if i < served {
				rows = append(rows, ref)
			}
		}
		counting := storetest.NewCounting(mem)
		r, err := visibility.NewRollups(set, counting, visibility.AllowAllReadQueries{}, visibility.NopRedactor{})
		if err != nil {
			t.Fatal(err)
		}
		got := r.Values(context.Background(), "todo", rows, store.TrivialScope(), nil)
		if len(got) != served {
			t.Fatalf("values for %d rows, want %d", len(got), served)
		}
		return counting.Cost()
	}
	if a, b := page(t, 100, 10).Calls, page(t, 100, 50).Calls; a != b {
		t.Errorf("calls at 10 rows = %d, at 50 rows = %d; want equal", a, b)
	}
	storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
		return page(t, n, 10), ""
	})
}
