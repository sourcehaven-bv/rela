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

const probeListMeta = `version: "1.0"
entities:
  action:
    label: Action
    id_prefix: "ACT-"
    properties:
      title: {type: string}
      status: {type: enum, values: [open, done], default: open}
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
      defer_date: {type: date}
      held:
        type: boolean
        rollup:
          exists: true
          along: [child_of*]
          max_depth: 4
          condition:
            project: "entity.status == 'on_hold'"
      held_by_tag:
        type: boolean
        rollup:
          exists: true
          along: [tagged, tag_parent*]
          max_depth: 4
          condition:
            tag: "entity.status == 'on_hold'"
      waiting:
        type: boolean
        rollup:
          exists: true
          siblings_before: child_of
          where: {status: open}
          when_parent: {ordering: sequential}
      deferred_above:
        type: boolean
        rollup:
          exists: true
          along: [child_of*]
          max_depth: 4
          condition:
            project: "entity.defer_date ~= nil and entity.defer_date > '2026-06-01'"
  project:
    label: Project
    id_prefix: "PRJ-"
    properties:
      title: {type: string}
      status: {type: enum, values: [active, on_hold], default: active}
      ordering: {type: enum, values: [parallel, sequential], default: parallel}
      defer_date: {type: date}
  tag:
    label: Tag
    id_prefix: "TAG-"
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
    orderable: incoming
  tagged:
    from: [action]
    to: [tag]
  tag_parent:
    from: [tag]
    to: [tag]
    max_outgoing: 1
  shared-with:
    from: [person]
    to: [project]
`

const (
	probeAvailable = "entity.held == false and entity.held_by_tag == false and entity.waiting == false" +
		" and entity.status == 'open'"
	probeAvailableGo = probeAvailable + " and (entity.defer_date == nil or entity.defer_date <= '2100-01-01')"
	probeDeferred    = probeAvailable + " and entity.deferred_above == false" +
		" and (entity.defer_date == nil or entity.defer_date <= '2026-06-01')"
)

// TestListCondition_ProbeLowers pins design gap 3.5 step 2 on the probe's
// Available list: its condition lowers to roll-up leaves when no role has
// a grant on the order field, and adding the defer conditions keeps the Go
// path. available_go adds a condition true on every row that keeps the Go
// path, so the pushed and Go rows are compared per reader. PRJ-2 is on
// hold but not shared with PERS-B, so ACT-3 is available to PERS-B only.
// ACT-2 is held by its tag, ACT-4 waits behind ACT-5, and ACT-5 is
// deferred by its own date and by PRJ-3's.
func TestListCondition_ProbeLowers(t *testing.T) {
	app, d := newProbeListApp(t)
	for _, tc := range []struct {
		list   string
		lowers bool
		want   map[string][]string
	}{
		{"available", true, map[string][]string{"PERS-A": {"ACT-1", "ACT-5"}, "PERS-B": {"ACT-1", "ACT-3", "ACT-5"}}},
		{"deferred", false, map[string][]string{"PERS-A": {"ACT-1"}, "PERS-B": {"ACT-1", "ACT-3"}}},
		{"available_go", false, map[string][]string{"PERS-A": {"ACT-1", "ACT-5"}, "PERS-B": {"ACT-1", "ACT-3", "ACT-5"}}},
	} {
		for user, want := range tc.want {
			t.Run(tc.list+"/"+user, func(t *testing.T) {
				ctx := gateCtxFor(principalCtx(user), t, d)
				query := map[string][]string{"list_id": {tc.list}, "sort": {"title"}}
				_, n, err := resolveListNarrowing(ctx, app, app.State(), "action", query)
				if err != nil {
					t.Fatal(err)
				}
				gated, err := gateAppListQuery(ctx, app, app.State(), "action", query)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, ok := n.pushdownPlan(ctx, "action", gated, 1, 50); ok != tc.lowers {
					t.Fatalf("pushdown = %v, want %v", ok, tc.lowers)
				}
				resp, rec := listEntitiesAs(principalCtx(user), t, app, d, "action", "actions",
					"list_id="+tc.list+"&sort=title")
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
}

// newProbeListApp builds the probe's actions, projects and tags:
//
//	ACT-1 in PRJ-1 (active, parallel)
//	ACT-2 in PRJ-1, tagged TAG-1 under the on-hold TAG-2
//	ACT-3 in PRJ-2 (on hold, not shared with PERS-B)
//	ACT-5 then ACT-4 in PRJ-3 (sequential, deferred past the cut); ACT-5
//	  is deferred past the cut too
//
// PERS-A reads everything; PERS-B reads actions, tags and people, and the
// projects shared with them.
func newProbeListApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(probeListMeta))
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
	mk("PRJ-1", "project", nil)
	mk("PRJ-2", "project", map[string]any{"status": "on_hold"})
	mk("PRJ-3", "project", map[string]any{"ordering": "sequential", "defer_date": "2026-09-01"})
	mk("TAG-1", "tag", nil)
	mk("TAG-2", "tag", map[string]any{"status": "on_hold"})
	for _, id := range []string{"ACT-1", "ACT-2", "ACT-3", "ACT-4"} {
		mk(id, "action", map[string]any{"status": "open"})
	}
	mk("ACT-5", "action", map[string]any{"status": "open", "defer_date": "2026-09-01"})
	rel("ACT-1", "child_of", "PRJ-1", nil)
	rel("ACT-2", "child_of", "PRJ-1", nil)
	rel("ACT-2", "tagged", "TAG-1", nil)
	rel("TAG-1", "tag_parent", "TAG-2", nil)
	rel("ACT-3", "child_of", "PRJ-2", nil)
	rel("ACT-5", "child_of", "PRJ-3", map[string]any{metamodel.OrderPropertyIn: 1})
	rel("ACT-4", "child_of", "PRJ-3", map[string]any{metamodel.OrderPropertyIn: 2})
	rel("PERS-B", "shared-with", "PRJ-1", nil)
	rel("PERS-B", "shared-with", "PRJ-3", nil)
	list := func(cond string) dataentryconfig.List {
		return dataentryconfig.List{
			EntityType: "action",
			Columns:    []dataentryconfig.ListColumn{{Property: "title"}},
			Condition:  cond,
		}
	}
	cfg := &Config{
		App: dataentryconfig.AppConfig{Name: "probe"},
		Lists: map[string]dataentryconfig.List{
			"available": list(probeAvailable), "deferred": list(probeDeferred), "available_go": list(probeAvailableGo),
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
			"base":   {Read: []string{"action", "tag", "person"}},
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
	return app, d
}
