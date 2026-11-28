package visibility_test

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// rollupAlongMeta is the probe's held roll-up twice: held lowers, so the
// store judges it; held_go adds a date comparison no project meets, which
// does not lower, so Go judges it. Both must agree for every reader.
const rollupAlongMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done], default: open}
      held:
        type: boolean
        rollup:
          exists: true
          along: [child_of*, in_folder*]
          max_depth: 12
          condition:
            project: "entity.status == 'on_hold' or entity.status == 'dropped'"
            folder: "entity.status == 'dropped'"
      held_go:
        type: boolean
        rollup:
          exists: true
          along: [child_of*, in_folder*]
          max_depth: 12
          condition:
            project: "entity.status == 'on_hold' or entity.status == 'dropped' or (entity.due ~= nil and entity.due < '1900-01-01')"
            folder: "entity.status == 'dropped'"
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      status: {type: enum, values: [active, on_hold, dropped], default: active}
      owner: {type: string}
      due: {type: date}
  folder:
    label: Folder
    id_prefix: "FLD-"
    properties:
      status: {type: enum, values: [active, dropped]}
relations:
  child_of:
    from: [action]
    to: [action, project]
    max_outgoing: 1
  in_folder:
    from: [project, folder]
    to: [folder]
    max_outgoing: 1
`

// typeFields is a [visibility.FieldScoper] with one scope per type.
type typeFields map[string]*search.FieldScope

func (f typeFields) SearchFieldScope(_ context.Context, typ string) (*search.FieldScope, error) {
	if fs, ok := f[typ]; ok {
		return fs, nil
	}
	return &search.FieldScope{}, nil
}

func compileAlongSet(t *testing.T) *rollup.Set {
	t.Helper()
	meta, err := metamodel.Parse([]byte(rollupAlongMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	cols := map[string]bool{}
	for _, c := range set.For("action") {
		cols[c.Property] = c.Along.Lowers()
	}
	if !cols["held"] || cols["held_go"] {
		t.Fatalf("lowers = %v, want held to lower and held_go not", cols)
	}
	return set
}

// ownedProjects reads only the projects owned by "me".
var ownedProjects = visibility.ReadQueryFunc(func(_ context.Context, typ string) (acl.ReadQueryResult, error) {
	if typ != "project" {
		return acl.ReadQueryResult{AllowAll: true}, nil
	}
	return acl.ReadQueryResult{Query: &store.GraphQuery{Props: []store.PropPredicate{
		{Property: "owner", Op: store.PropEqual, Value: "me", Scalar: true},
	}}}, nil
})

// TestRollups_AlongACL pins design section 3.6 for along roll-ups: a hidden
// project is not visited and breaks the walk above it, a project status a
// `visible:` grant hides reads as unset (never as its default), and a
// roll-up hidden on the source is absent. The store-judged and the
// Go-judged column agree in every case.
func TestRollups_AlongACL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	set := compileAlongSet(t)
	st := memstore.New()
	mk := func(id, typ string, props map[string]any) {
		e := entity.New(id, typ)
		maps.Copy(e.Properties, props)
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	rel := func(from, typ, to string) {
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil); err != nil {
			t.Fatal(err)
		}
	}
	// ACT-1 in PRJ-1 (on hold, not mine); ACT-2 in PRJ-2 (active, not
	// mine) under the dropped FLD-1; ACT-3 in PRJ-3 (on hold, mine); ACT-4
	// in PRJ-4 (no status, mine).
	mk("PRJ-1", "project", map[string]any{"status": "on_hold", "owner": "other"})
	mk("PRJ-2", "project", map[string]any{"status": "active", "owner": "other"})
	mk("PRJ-3", "project", map[string]any{"status": "on_hold", "owner": "me"})
	mk("PRJ-4", "project", map[string]any{"owner": "me"})
	mk("FLD-1", "folder", map[string]any{"status": "dropped"})
	rel("PRJ-2", "in_folder", "FLD-1")
	var rows []entity.Ref
	for i := 1; i <= 4; i++ {
		id := fmt.Sprintf("ACT-%d", i)
		mk(id, "action", nil)
		rel(id, "child_of", fmt.Sprintf("PRJ-%d", i))
		rows = append(rows, entity.Ref{ID: id})
	}
	statusHidden := typeFields{"project": {Gated: []string{"status"}}}
	statusShownWhenMine := typeFields{"project": {Gated: []string{"status"}, Grants: []search.FieldGrant{{
		Fields: []string{"status"},
		When: &store.GraphQuery{Props: []store.PropPredicate{
			{Property: "owner", Op: store.PropEqual, Value: "me", Scalar: true},
		}},
	}}}}
	heldHidden := typeFields{"action": {Gated: []string{"held", "held_go"}}}
	cases := []struct {
		name   string
		rows   visibility.ReadQueryProvider
		fields visibility.FieldScoper
		want   string
	}{
		{"owner", visibility.AllowAllReadQueries{}, visibility.NopRedactor{}, "true true true false"},
		{"reader of own projects", ownedProjects, visibility.NopRedactor{}, "false false true false"},
		{"status hidden", visibility.AllowAllReadQueries{}, statusHidden, "false true false false"},
		{"status shown on own projects", visibility.AllowAllReadQueries{}, statusShownWhenMine,
			"false true true false"},
		{"roll-up hidden on the source", visibility.AllowAllReadQueries{}, heldHidden, "- - - -"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := visibility.NewRollups(set, st, tc.rows, tc.fields)
			if err != nil {
				t.Fatal(err)
			}
			got := r.Values(ctx, "action", rows, store.TrivialScope(), nil)
			for _, prop := range []string{"held", "held_go"} {
				cells := make([]string, len(rows))
				for i, ref := range rows {
					cells[i] = "-"
					if v, ok := got[ref.ID][prop]; ok {
						cells[i] = fmt.Sprint(v)
					}
				}
				if line := strings.Join(cells, " "); line != tc.want {
					t.Errorf("%s = %q, want %q", prop, line, tc.want)
				}
			}
		})
	}
}

// TestRollups_AlongBudget pins the cost of an along column on memstore: the
// same store calls for a page of 10 and of 50 sources, and the same calls
// and rows over a store padded with 100 or 1,000 more actions, projects and
// folders linked by the relations the walk reads.
func TestRollups_AlongBudget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	set := compileAlongSet(t)
	build := func(t *testing.T, page, pad int) (*storetest.Counting, []entity.Ref) {
		t.Helper()
		st := storetest.NewCounting(memstore.New())
		mk := func(id, typ string, props map[string]any) {
			e := entity.New(id, typ)
			maps.Copy(e.Properties, props)
			if err := st.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		rel := func(from, typ, to string) {
			if _, err := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil); err != nil {
				t.Fatal(err)
			}
		}
		mk("FLD-1", "folder", map[string]any{"status": "active"})
		mk("FLD-2", "folder", map[string]any{"status": "dropped"})
		rel("FLD-1", "in_folder", "FLD-2")
		var rows []entity.Ref
		for i := range page {
			act, grp, prj := fmt.Sprintf("ACT-%d", i), fmt.Sprintf("ACT-G%d", i), fmt.Sprintf("PRJ-%d", i)
			mk(act, "action", nil)
			mk(grp, "action", nil)
			mk(prj, "project", map[string]any{"status": "active"})
			rel(act, "child_of", grp)
			rel(grp, "child_of", prj)
			rel(prj, "in_folder", "FLD-1")
			rows = append(rows, entity.Ref{ID: act})
		}
		for i := range pad {
			act, prj, fld := fmt.Sprintf("ACT-P%d", i), fmt.Sprintf("PRJ-P%d", i), fmt.Sprintf("FLD-P%d", i)
			mk(act, "action", nil)
			mk(prj, "project", map[string]any{"status": "on_hold"})
			mk(fld, "folder", map[string]any{"status": "dropped"})
			rel(act, "child_of", prj)
			rel(prj, "in_folder", fld)
		}
		st.Reset()
		return st, rows
	}
	measure := func(t *testing.T, prop string, page, pad int) (storetest.Cost, string) {
		t.Helper()
		st, rows := build(t, page, pad)
		r, err := visibility.NewRollups(set, st, ownedProjectsAll, visibility.NopRedactor{})
		if err != nil {
			t.Fatal(err)
		}
		got := r.Values(ctx, "action", rows, store.TrivialScope(), func(p string) bool { return p == prop })
		for _, ref := range rows {
			if got[ref.ID][prop] != true {
				t.Fatalf("%s on %s = %v, want true", prop, ref.ID, got[ref.ID][prop])
			}
		}
		return st.Cost(), st.String()
	}
	for _, prop := range []string{"held", "held_go"} {
		t.Run(prop, func(t *testing.T) {
			t.Parallel()
			small, smallDetail := measure(t, prop, 10, 0)
			large, largeDetail := measure(t, prop, 50, 0)
			if small.Calls != large.Calls {
				t.Errorf("calls grow with the page: %d at 10, %d at 50 (%s | %s)",
					small.Calls, large.Calls, smallDetail, largeDetail)
			}
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				return measure(t, prop, 10, n)
			})
		})
	}
}

// ownedProjectsAll is a read query provider that gates every type with a
// store query, so the budget includes the gate reads.
var ownedProjectsAll = visibility.ReadQueryFunc(func(context.Context, string) (acl.ReadQueryResult, error) {
	return acl.ReadQueryResult{Query: &store.GraphQuery{}}, nil
})
