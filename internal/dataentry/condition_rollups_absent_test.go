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

// absentRollupsACL hides `_order_in` from PERS-B on ACT-4's child_of edge
// through a relation `visible:` grant, so PERS-B's waiting roll-up is
// withheld on PRJ-2's children ACT-3 and ACT-4.
const absentRollupsACL = `roles:
  all:
    read: ["*"]
    relations:
      action:
        - relation: child_of
          visible: [{field: _order_in}]
  most:
    read: ["*"]
    relations:
      action:
        - relation: child_of
          visible: [{field: _order_in}]
          when: "entity.title ~= 'ACT-4'"
assignments:
  PERS-A: all
  PERS-B: most
`

// TestListCondition_AbsentRollupFailsClosed pins that a list condition on
// a roll-up the reader cannot compute matches no row, whatever it does
// with the value: `== false`, `not (== true)` and `~= true` alike. ACT-1
// waits for nothing and ACT-2 waits for ACT-1, for both readers; PERS-B's
// value on ACT-3 and ACT-4 is withheld.
func TestListCondition_AbsentRollupFailsClosed(t *testing.T) {
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
	mk("PERS-A", "person", nil)
	mk("PERS-B", "person", nil)
	mk("PRJ-1", "project", map[string]any{"ordering": "sequential"})
	mk("PRJ-2", "project", map[string]any{"ordering": "sequential"})
	for i := 1; i <= 4; i++ {
		act := fmt.Sprintf("ACT-%d", i)
		mk(act, "action", nil)
		if _, cerr := st.CreateRelation(ctx,
			entity.RelationKey{From: act, Type: "child_of", To: fmt.Sprintf("PRJ-%d", (i+1)/2)},
			&store.RelationData{Properties: map[string]any{metamodel.OrderPropertyIn: i}}); cerr != nil {
			t.Fatal(cerr)
		}
	}
	conditions := map[string]string{
		"free":     "entity.waiting == false",
		"not_held": "not (entity.waiting == true)",
		"not_eq":   "entity.waiting ~= true",
	}
	lists := map[string]dataentryconfig.List{}
	for id, cond := range conditions {
		lists[id] = dataentryconfig.List{
			EntityType: "action",
			Columns:    []dataentryconfig.ListColumn{{Property: "title"}, {Property: "waiting"}},
			Condition:  cond,
		}
	}
	cfg := &Config{
		App:        dataentryconfig.AppConfig{Name: "absent"},
		Lists:      lists,
		Views:      map[string]dataentryconfig.ViewConfig{},
		Forms:      map[string]dataentryconfig.Form{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture(), appbuildtest.WithStore(st))
	policy, err := acl.ParsePolicy([]byte(absentRollupsACL), "acl.yaml")
	if err != nil {
		t.Fatal(err)
	}
	d := mustNewACL(t, policy, app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", meta, app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	wireRealViewConditions(t, app)
	want := map[string][]string{
		"PERS-A": {"ACT-1", "ACT-3"},
		"PERS-B": {"ACT-1"},
	}
	for user, ids := range want {
		for list := range conditions {
			t.Run(user+"/"+list, func(t *testing.T) {
				resp, rec := listEntitiesAs(principalCtx(user), t, app, d, "action", "actions",
					"list_id="+list+"&sort=title")
				if rec.Code != http.StatusOK {
					t.Fatalf("list = %d %s", rec.Code, rec.Body)
				}
				var got []string
				for _, row := range resp.Data {
					got = append(got, row.ID)
				}
				if fmt.Sprint(got) != fmt.Sprint(ids) || resp.Meta.Total != len(ids) {
					t.Errorf("rows = %v (total %d), want %v", got, resp.Meta.Total, ids)
				}
			})
		}
	}
}
