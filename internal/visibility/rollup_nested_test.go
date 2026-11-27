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
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// rollupNestedMeta is the Stalled probe of design section 10.1: a project
// has an available action when a direct child is open, not waiting, and
// neither held through its parents nor through its tags.
const rollupNestedMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done], default: open}
      owner: {type: string}
      waiting: {type: boolean, default: false}
      held:
        type: boolean
        rollup:
          exists: true
          along: [child_of*]
          max_depth: 6
          condition: {project: "entity.status == 'on_hold'"}
      held_by_tag:
        type: boolean
        rollup:
          exists: true
          along: [tagged, tag_parent*]
          max_depth: 6
          condition: {tag: "entity.status == 'on_hold'"}
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      status: {type: enum, values: [active, on_hold], default: active}
      has_available_action:
        type: boolean
        rollup:
          exists: true
          via: parent_of
          condition:
            action: >-
              entity.status == 'open' and entity.waiting == false
              and entity.held == false and entity.held_by_tag == false
  tag:
    label: Tag
    id_prefix: "TAG-"
    properties:
      status: {type: enum, values: [active, on_hold], default: active}
relations:
  child_of:
    from: [action]
    to: [action, project]
    max_outgoing: 1
    inverse: parent_of
  tagged:
    from: [action]
    to: [tag]
  tag_parent:
    from: [tag]
    to: [tag]
    max_outgoing: 1
`

func compileNestedSet(t *testing.T) *rollup.Set {
	t.Helper()
	meta, err := metamodel.Parse([]byte(rollupNestedMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// ownActions reads only the actions owned by "me".
var ownActions = visibility.ReadQueryFunc(func(_ context.Context, typ string) (acl.ReadQueryResult, error) {
	if typ != "action" {
		return acl.ReadQueryResult{AllowAll: true}, nil
	}
	return acl.ReadQueryResult{Query: &store.GraphQuery{Props: []store.PropPredicate{
		{Property: "owner", Op: store.PropEqual, Value: "me", Scalar: true},
	}}}, nil
})

type nestedSeed struct {
	t  *testing.T
	st store.Store
}

func (s nestedSeed) mk(id, typ string, props map[string]any) {
	s.t.Helper()
	e := entity.New(id, typ)
	maps.Copy(e.Properties, props)
	if err := s.st.CreateEntity(s.t.Context(), e); err != nil {
		s.t.Fatal(err)
	}
}

func (s nestedSeed) rel(from, typ, to string) {
	s.t.Helper()
	if _, err := s.st.CreateRelation(s.t.Context(), entity.RelationKey{From: from, Type: typ, To: to},
		nil); err != nil {
		s.t.Fatal(err)
	}
}

func (s nestedSeed) action(id, parent string, props map[string]any) {
	s.t.Helper()
	p := map[string]any{"owner": "me"}
	maps.Copy(p, props)
	s.mk(id, "action", p)
	s.rel(id, "child_of", parent)
}

// seedStalled seeds the projects of design section 10.8 and returns them
// in order:
//
//	PRJ-1: one open child, one done: available.
//	PRJ-2: a child held by its tag, one waiting, one done: stalled.
//	PRJ-3: only an open group, whose own child is done: the group counts.
//	PRJ-4: an open child owned by "other", one waiting.
//	PRJ-5: a child whose tag's parent tag is on hold: stalled.
//	PRJ-6: no children: stalled.
func seedStalled(t *testing.T, st store.Store) []entity.Ref {
	t.Helper()
	s := nestedSeed{t: t, st: st}
	var rows []entity.Ref
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("PRJ-%d", i)
		s.mk(id, "project", map[string]any{"status": "active"})
		rows = append(rows, entity.Ref{ID: id})
	}
	s.mk("TAG-H", "tag", map[string]any{"status": "on_hold"})
	s.mk("TAG-C", "tag", map[string]any{"status": "active"})
	s.rel("TAG-C", "tag_parent", "TAG-H")
	s.action("ACT-1A", "PRJ-1", nil)
	s.action("ACT-1B", "PRJ-1", map[string]any{"status": "done"})
	s.action("ACT-2A", "PRJ-2", nil)
	s.rel("ACT-2A", "tagged", "TAG-H")
	s.action("ACT-2B", "PRJ-2", map[string]any{"waiting": true})
	s.action("ACT-2C", "PRJ-2", map[string]any{"status": "done"})
	s.action("ACT-3G", "PRJ-3", nil)
	s.action("ACT-3C", "ACT-3G", map[string]any{"status": "done"})
	s.action("ACT-4A", "PRJ-4", map[string]any{"owner": "other"})
	s.action("ACT-4B", "PRJ-4", map[string]any{"waiting": true})
	s.action("ACT-5A", "PRJ-5", nil)
	s.rel("ACT-5A", "tagged", "TAG-C")
	return rows
}

func valueLine(got map[string]map[string]any, rows []entity.Ref, prop string) string {
	parts := make([]string, len(rows))
	for i, ref := range rows {
		v, ok := got[ref.ID][prop]
		if !ok {
			parts[i] = "-"
			continue
		}
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, " ")
}

// assertNestedRollups pins the semantics and ACL of design section 10.8 on
// st: an available child or group makes a project not stalled; held,
// waiting and done children do not; a child the reader cannot read does
// not count for that reader; an inner roll-up `visible:` hides makes the
// outer value absent; and a row cap over the whole nested evaluation
// leaves the value out.
func assertNestedRollups(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	set := compileNestedSet(t)
	rows := seedStalled(t, st)
	innerHidden := typeFields{"action": {Gated: []string{"held_by_tag"}}}
	innerShownWhenMine := typeFields{"action": {Gated: []string{"held_by_tag"}, Grants: []search.FieldGrant{{
		Fields: []string{"held_by_tag"},
		When: &store.GraphQuery{Props: []store.PropPredicate{
			{Property: "status", Op: store.PropEqual, Value: "done", Scalar: true},
		}},
	}}}}
	cases := []struct {
		name   string
		rows   visibility.ReadQueryProvider
		fields visibility.FieldScoper
		cap    int
		want   string
	}{
		{"owner", visibility.AllowAllReadQueries{}, visibility.NopRedactor{}, 0, "true false true true false false"},
		{"reader of own actions", ownActions, visibility.NopRedactor{}, 0, "true false true false false false"},
		{"inner hidden", visibility.AllowAllReadQueries{}, innerHidden, 0, "- - - - - false"},
		{"inner shown on done rows only", visibility.AllowAllReadQueries{}, innerShownWhenMine, 0,
			"- - - - - false"},
		{"row cap over the nested evaluation", visibility.AllowAllReadQueries{}, visibility.NopRedactor{}, 8,
			"- - - - - -"},
		{"row cap shared with the inner walks", visibility.AllowAllReadQueries{}, visibility.NopRedactor{}, 20,
			"- - - - - -"},
		{"row cap with room", visibility.AllowAllReadQueries{}, visibility.NopRedactor{}, 400,
			"true false true true false false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := visibility.NewRollups(set, st, tc.rows, tc.fields)
			if err != nil {
				t.Fatal(err)
			}
			got := r.WithRowCap(tc.cap).Values(ctx, "project", rows, store.TrivialScope(), nil)
			if line := valueLine(got, rows, "has_available_action"); line != tc.want {
				t.Errorf("has_available_action = %q, want %q", line, tc.want)
			}
		})
	}
}

// TestRollups_Nested runs the nested roll-up conformance on memstore and
// fsstore; the sqlite and postgres twins run the same assertions. There is
// one evaluation path, so the backends are what it is compared across.
func TestRollups_Nested(t *testing.T) {
	t.Parallel()
	t.Run("memstore", func(t *testing.T) {
		t.Parallel()
		assertNestedRollups(t, memstore.New())
	})
	t.Run("fsstore", func(t *testing.T) {
		t.Parallel()
		fs := storage.NewMemFS()
		rooted, err := storage.NewRootedFS(fs, "/")
		if err != nil {
			t.Fatal(err)
		}
		schemas := map[string]store.EntityTypeSchema{}
		for _, typ := range []string{"action", "project", "tag"} {
			schemas[typ] = store.EntityTypeSchema{Plural: typ + "s"}
		}
		st, err := fsstore.New(fsstore.Config{
			FS: fs, Rooted: rooted, EntitiesKey: "entities", RelationsKey: "relations",
			AttachmentsKey: "attachments", CacheKey: ".rela", Schemas: schemas,
		})
		if err != nil {
			t.Fatal(err)
		}
		assertNestedRollups(t, st)
	})
}

// TestRollups_NestedBudget pins the cost of a nested roll-up: one batched
// call per inner roll-up over the whole frontier, so a page of 10 projects
// and one of 50 make the same store calls.
func TestRollups_NestedBudget(t *testing.T) {
	t.Parallel()
	set := compileNestedSet(t)
	measure := func(t *testing.T, page int) (storetest.Cost, string) {
		t.Helper()
		st := storetest.NewCounting(memstore.New())
		s := nestedSeed{t: t, st: st}
		s.mk("TAG-H", "tag", map[string]any{"status": "on_hold"})
		var rows []entity.Ref
		for i := range page {
			prj := fmt.Sprintf("PRJ-%d", i)
			s.mk(prj, "project", nil)
			s.action(fmt.Sprintf("ACT-%dA", i), prj, nil)
			s.action(fmt.Sprintf("ACT-%dB", i), prj, nil)
			s.rel(fmt.Sprintf("ACT-%dB", i), "tagged", "TAG-H")
			rows = append(rows, entity.Ref{ID: prj})
		}
		st.Reset()
		r, err := visibility.NewRollups(set, st, ownedProjectsAll, visibility.NopRedactor{})
		if err != nil {
			t.Fatal(err)
		}
		got := r.Values(t.Context(), "project", rows, store.TrivialScope(), nil)
		for _, ref := range rows {
			if got[ref.ID]["has_available_action"] != true {
				t.Fatalf("has_available_action on %s = %v", ref.ID, got[ref.ID]["has_available_action"])
			}
		}
		return st.Cost(), st.String()
	}
	small, smallDetail := measure(t, 10)
	large, largeDetail := measure(t, 50)
	if small.Calls != large.Calls {
		t.Errorf("calls grow with the page: %d at 10, %d at 50 (%s | %s)",
			small.Calls, large.Calls, smallDetail, largeDetail)
	}
}
