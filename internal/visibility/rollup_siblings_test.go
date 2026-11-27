package visibility_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
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

const rollupSiblingsMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      status: {type: enum, values: [open, done], default: open}
      secret: {type: string}
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
      waiting:
        type: boolean
        rollup:
          exists: true
          siblings_before: child_of
          where: {status: open}
          when_parent: {ordering: sequential}
          climb: true
          max_depth: 4
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
relations:
  child_of:
    from: [action]
    to: [action, project]
    max_outgoing: 1
    orderable: incoming
`

// orderHidden is a field scoper that hides nothing on entities and hides
// _order_in on the child_of edges from the ids it holds.
type orderHidden []string

func (orderHidden) SearchFieldScope(context.Context, string) (*search.FieldScope, error) {
	return &search.FieldScope{}, nil
}

func (orderHidden) RelationMetaGated(relType string) bool { return relType == "child_of" }

func (h orderHidden) RelationFieldVerdicts(
	_ context.Context, from *entity.Entity, relType string, keys []string,
) map[string]bool {
	if relType != "child_of" || !slices.Contains(h, from.ID) {
		return nil
	}
	out := map[string]bool{}
	for _, k := range keys {
		out[k] = false
	}
	return out
}

// secretActions reads only the actions not marked secret.
var secretActions = visibility.ReadQueryFunc(func(_ context.Context, typ string) (acl.ReadQueryResult, error) {
	if typ != "action" {
		return acl.ReadQueryResult{AllowAll: true}, nil
	}
	return acl.ReadQueryResult{Query: &store.GraphQuery{Props: []store.PropPredicate{
		{Property: "secret", Op: store.PropNotEqualOrEmpty, Value: "yes", Scalar: true},
	}}}, nil
})

// TestRollups_SiblingsACL pins design section 4.3's gating: a sibling the
// reader cannot read does not make anyone wait, and its edge's order is
// never judged; a readable sibling whose order is hidden leaves the
// roll-up absent for every child of that parent, and only of that parent.
func TestRollups_SiblingsACL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	meta, err := metamodel.Parse([]byte(rollupSiblingsMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	mk := func(id, typ string, props map[string]any) {
		e := entity.New(id, typ)
		maps.Copy(e.Properties, props)
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	child := func(from, to string, order int) {
		data := &store.RelationData{Properties: map[string]any{metamodel.OrderPropertyIn: order}}
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: "child_of", To: to}, data); err != nil {
			t.Fatal(err)
		}
	}
	mk("PRJ-1", "project", map[string]any{"ordering": "sequential"})
	mk("PRJ-2", "project", map[string]any{"ordering": "sequential"})
	mk("ACT-1", "action", map[string]any{"secret": "yes"})
	mk("ACT-2", "action", nil)
	mk("ACT-3", "action", nil)
	mk("ACT-4", "action", nil)
	child("ACT-1", "PRJ-1", 1)
	child("ACT-2", "PRJ-1", 2)
	child("ACT-3", "PRJ-2", 1)
	child("ACT-4", "PRJ-2", 2)
	rows := []entity.Ref{{ID: "ACT-2"}, {ID: "ACT-3"}, {ID: "ACT-4"}}
	cases := []struct {
		name   string
		rows   visibility.ReadQueryProvider
		fields visibility.FieldScoper
		want   map[string]any
	}{
		{"owner", visibility.AllowAllReadQueries{}, visibility.NopRedactor{},
			map[string]any{"ACT-2": true, "ACT-3": false, "ACT-4": true}},
		{"hidden_sibling", secretActions, visibility.NopRedactor{},
			map[string]any{"ACT-2": false, "ACT-3": false, "ACT-4": true}},
		{"hidden_sibling_hidden_order", secretActions, orderHidden{"ACT-1"},
			map[string]any{"ACT-2": false, "ACT-3": false, "ACT-4": true}},
		{"visible_sibling_hidden_order", secretActions, orderHidden{"ACT-3"},
			map[string]any{"ACT-2": false}},
		{"owner_hidden_order", visibility.AllowAllReadQueries{}, orderHidden{"ACT-1"},
			map[string]any{"ACT-3": false, "ACT-4": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := visibility.NewRollups(set, st, tc.rows, tc.fields)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]any{}
			for id, props := range r.Values(ctx, "action", rows, store.TrivialScope(), nil) {
				if v, ok := props["waiting"]; ok {
					got[id] = v
				}
			}
			if !maps.Equal(got, tc.want) {
				t.Fatalf("waiting = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRollups_SiblingsScaleFree pins the cost of a page's earlier-siblings
// column: the rows read grow with the siblings of the page's parents, never
// with the parents and children elsewhere in the store.
func TestRollups_SiblingsScaleFree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	meta, err := metamodel.Parse([]byte(rollupSiblingsMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	storetest.AssertScaleFree(t, func(pad int) (storetest.Cost, string) {
		st := storetest.NewCounting(memstore.New())
		mk := func(id, typ string, props map[string]any) {
			e := entity.New(id, typ)
			maps.Copy(e.Properties, props)
			if err := st.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		child := func(from, to string, order int) {
			data := &store.RelationData{Properties: map[string]any{metamodel.OrderPropertyIn: order}}
			if _, err := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: "child_of", To: to}, data); err != nil {
				t.Fatal(err)
			}
		}
		mk("PRJ-0", "project", map[string]any{"ordering": "sequential"})
		for i := range 3 {
			id := fmt.Sprintf("ACT-0%d", i)
			mk(id, "action", nil)
			child(id, "PRJ-0", i)
		}
		for i := range pad {
			prj, act := fmt.Sprintf("PRJ-%d", i+1), fmt.Sprintf("ACT-%d", i+10)
			mk(prj, "project", map[string]any{"ordering": "sequential"})
			mk(act, "action", nil)
			child(act, prj, 1)
		}
		r, err := visibility.NewRollups(set, st, secretActions, orderHidden{"ACT-99999"})
		if err != nil {
			t.Fatal(err)
		}
		st.Reset()
		rows := []entity.Ref{{ID: "ACT-00"}, {ID: "ACT-01"}, {ID: "ACT-02"}}
		got := r.Values(ctx, "action", rows, store.TrivialScope(), nil)
		if len(got) != 3 || got["ACT-02"]["waiting"] != true {
			t.Fatalf("waiting = %v", got)
		}
		return st.Cost(), st.String()
	})
}
