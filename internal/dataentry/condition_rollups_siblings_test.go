package dataentry

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const siblingsListMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      title: {type: string}
      status: {type: enum, values: [open, done], default: open}
      waiting:
        type: boolean
        rollup:
          exists: true
          siblings_before: child_of
          where: {status: open}
          when_parent: {ordering: sequential}
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      title: {type: string}
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      title: {type: string}
relations:
  child_of:
    from: [action]
    to: [project]
    max_outgoing: 1
    orderable: incoming
  shared-with:
    from: [person]
    to: [action]
`

// TestListCondition_ReadsSiblingsRollups pins that slice A's Go path for a
// list `condition:` judges an earlier-siblings roll-up as the reader
// computes it. ACT-1 comes before ACT-2 but is not shared with PERS-B, so
// ACT-2 waits for PERS-A only.
func TestListCondition_ReadsSiblingsRollups(t *testing.T) {
	meta, err := metamodel.Parse([]byte(siblingsListMeta))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	ctx := context.Background()
	mk := func(id, typ string, props map[string]any) {
		e := entity.New(id, typ)
		e.Properties["title"] = id
		maps.Copy(e.Properties, props)
		if cerr := st.CreateEntity(ctx, e); cerr != nil {
			t.Fatal(cerr)
		}
	}
	rel := func(from, typ, to string, props map[string]any) {
		if _, cerr := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to},
			&store.RelationData{Properties: props}); cerr != nil {
			t.Fatal(cerr)
		}
	}
	mk("PERS-A", "person", nil)
	mk("PERS-B", "person", nil)
	mk("PRJ-1", "project", map[string]any{"ordering": "sequential"})
	mk("PRJ-2", "project", map[string]any{"ordering": "sequential"})
	for i := 1; i <= 4; i++ {
		act := fmt.Sprintf("ACT-%d", i)
		mk(act, "action", nil)
		rel(act, "child_of", fmt.Sprintf("PRJ-%d", (i+1)/2), map[string]any{metamodel.OrderPropertyIn: i})
	}
	for _, act := range []string{"ACT-2", "ACT-3", "ACT-4"} {
		rel("PERS-B", "shared-with", act, nil)
	}
	cfg := &Config{
		App: dataentryconfig.AppConfig{Name: "siblings"},
		Lists: map[string]dataentryconfig.List{
			"available": {
				EntityType: "action",
				Columns:    []dataentryconfig.ListColumn{{Property: "title"}, {Property: "waiting"}},
				Condition:  "entity.waiting == false",
			},
		},
		Views:      map[string]dataentryconfig.ViewConfig{},
		Forms:      map[string]dataentryconfig.Form{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture(), appbuildtest.WithStore(st))
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all":    {Read: []string{"*"}},
			"base":   {Read: []string{"project", "person"}},
			"shared": {Read: []string{"action"}},
		},
		Assignments:   map[string]string{"PERS-A": "all", "PERS-B": "base"},
		RoleRelations: map[string]acl.RoleRelationDef{"shared-with": {Confers: "shared"}},
	}, app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", meta, app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	wireRealViewConditions(t, app)
	for user, want := range map[string][]string{
		"PERS-A": {"ACT-1", "ACT-3"},
		"PERS-B": {"ACT-2", "ACT-3"},
	} {
		t.Run(user, func(t *testing.T) {
			resp, rec := listEntitiesAs(principalCtx(user), t, app, d, "action", "actions",
				"list_id=available&sort=title")
			if rec.Code != http.StatusOK {
				t.Fatalf("list = %d %s", rec.Code, rec.Body)
			}
			var got []string
			for _, row := range resp.Data {
				got = append(got, row.ID)
			}
			if fmt.Sprint(got) != fmt.Sprint(want) || resp.Meta.Total != len(want) {
				t.Errorf("rows = %v (total %d), want %v", got, resp.Meta.Total, want)
			}
		})
	}
}
