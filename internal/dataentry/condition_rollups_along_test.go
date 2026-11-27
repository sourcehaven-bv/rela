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
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const alongListMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      title: {type: string}
      held:
        type: boolean
        rollup:
          exists: true
          along: [child_of*]
          max_depth: 4
          condition:
            project: "entity.status == 'on_hold'"
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      title: {type: string}
      status: {type: enum, values: [active, on_hold], default: active}
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      title: {type: string}
relations:
  child_of:
    from: [action]
    to: [action, project]
    max_outgoing: 1
  shared-with:
    from: [person]
    to: [project]
`

// TestListCondition_ReadsAlongRollups pins that slice A's Go path for a list
// `condition:` judges an along roll-up as the reader computes it. ACT-2's
// on-hold project is not shared with PERS-B, so the walk does not visit it
// and ACT-2 is available to PERS-B only. ACT-3's project holds no status,
// which reads as the default active.
func TestListCondition_ReadsAlongRollups(t *testing.T) {
	meta, err := metamodel.Parse([]byte(alongListMeta))
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
	rel := func(from, typ, to string) {
		if _, cerr := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil); cerr != nil {
			t.Fatal(cerr)
		}
	}
	mk("PERS-A", "person", nil)
	mk("PERS-B", "person", nil)
	mk("PRJ-1", "project", map[string]any{"status": "on_hold"})
	mk("PRJ-2", "project", map[string]any{"status": "on_hold"})
	mk("PRJ-3", "project", nil)
	for i := 1; i <= 3; i++ {
		act, prj := fmt.Sprintf("ACT-%d", i), fmt.Sprintf("PRJ-%d", i)
		mk(act, "action", nil)
		rel(act, "child_of", prj)
	}
	rel("PERS-B", "shared-with", "PRJ-1")
	rel("PERS-B", "shared-with", "PRJ-3")
	cfg := &Config{
		App: dataentryconfig.AppConfig{Name: "along"},
		Lists: map[string]dataentryconfig.List{
			"available": {
				EntityType: "action",
				Columns:    []dataentryconfig.ListColumn{{Property: "title"}, {Property: "held"}},
				Condition:  "entity.held == false",
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
			"base":   {Read: []string{"action", "person"}},
			"shared": {Read: []string{"project"}},
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
		"PERS-A": {"ACT-3"},
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
