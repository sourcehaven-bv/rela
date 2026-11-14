package dataentry

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Roll-ups over a path of relations (TKT-LTSNHD) on the data-entry list.

const rollupPathWireMeta = `version: "1.0"
entities:
  eis:
    label: Eis
    id_prefix: "NE-"
    properties:
      title: {type: string}
      maatregel_aantal:
        type: integer
        rollup: { via: [verwijst_naar, wordt_geimplementeerd_door], count: true }
      gedekt:
        type: boolean
        rollup: { via: [verwijst_naar, wordt_geimplementeerd_door], exists: true, where: { status: actief } }
  iso:
    label: ISO
    id_prefix: "ISO-"
    properties:
      title: {type: string}
  maatregel:
    label: Maatregel
    id_prefix: "MAAT-"
    properties:
      title: {type: string}
      status: {type: enum, values: [actief, gepland]}
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      title: {type: string}
relations:
  verwijst_naar:
    from: [eis]
    to: [iso]
  implementeert:
    from: [maatregel]
    to: [iso]
    inverse: wordt_geimplementeerd_door
  shared-with:
    from: [person]
    to: [iso]
`

// rollupPathWirePolicy: PERS-A reads everything; PERS-B reads eis,
// maatregel and person, and only the ISO controls shared with it.
func rollupPathWirePolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all":      {Read: []string{"*"}},
			"base":     {Read: []string{"eis", "maatregel", "person"}},
			"isolezer": {Read: []string{"iso"}},
		},
		Assignments:   map[string]string{"PERS-A": "all", "PERS-B": "base"},
		RoleRelations: map[string]acl.RoleRelationDef{"shared-with": {Confers: "isolezer"}},
	}
}

// newRollupPathApp seeds n eisen, each with the same shape:
//   - NE-i -> ISO-i-1 (shared with PERS-B) and ISO-i-2 (not shared).
//   - MAAT-i-1 (actief) implements both, MAAT-i-2 (actief) only ISO-i-2,
//     MAAT-i-3 (gepland) only ISO-i-1.
//
// NE-0 has no controls.
func newRollupPathApp(t *testing.T, n int, base store.Store) (*App, *acl.Declarative) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(rollupPathWireMeta))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ctx := context.Background()
	mk := func(id, typ string, props map[string]any) {
		e := entity.New(id, typ)
		maps.Copy(e.Properties, props)
		if cerr := base.CreateEntity(ctx, e); cerr != nil {
			t.Fatal(cerr)
		}
	}
	rel := func(from, typ, to string) {
		if _, cerr := base.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil); cerr != nil {
			t.Fatal(cerr)
		}
	}
	for _, p := range []string{"PERS-A", "PERS-B"} {
		mk(p, "person", map[string]any{"title": p})
	}
	mk("NE-0", "eis", map[string]any{"title": "NE-0"})
	for i := 1; i <= n; i++ {
		eis := fmt.Sprintf("NE-%d", i)
		mk(eis, "eis", map[string]any{"title": eis})
		iso := func(j int) string { return fmt.Sprintf("ISO-%d-%d", i, j) }
		maat := func(j int) string { return fmt.Sprintf("MAAT-%d-%d", i, j) }
		for j := 1; j <= 2; j++ {
			mk(iso(j), "iso", map[string]any{"title": iso(j)})
			rel(eis, "verwijst_naar", iso(j))
		}
		rel("PERS-B", "shared-with", iso(1))
		for j, status := range []string{"actief", "actief", "gepland"} {
			mk(maat(j+1), "maatregel", map[string]any{"title": maat(j + 1), "status": status})
		}
		rel(maat(1), "implementeert", iso(1))
		rel(maat(1), "implementeert", iso(2))
		rel(maat(2), "implementeert", iso(2))
		rel(maat(3), "implementeert", iso(1))
	}
	cfg := &Config{
		App: dataentryconfig.AppConfig{Name: "rollup-paths"},
		Lists: map[string]dataentryconfig.List{
			"eisen": {EntityType: "eis", Columns: []dataentryconfig.ListColumn{
				{Property: "title"}, {Property: "maatregel_aantal"}, {Property: "gedekt"},
			}},
		},
		Views:      map[string]dataentryconfig.ViewConfig{},
		Forms:      map[string]dataentryconfig.Form{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
	app := newAppFromParts(cfg, meta, newFixture(), appbuildtest.WithStore(base))
	d := mustNewACL(t, rollupPathWirePolicy(), app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", meta, app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	return app, d
}

// TestRollups_PathPerPrincipal pins that a path roll-up counts each measure
// once and only through controls the reader may read: PERS-B cannot read
// ISO-i-2, so MAAT-i-2 is out of reach and MAAT-i-1 still counts through
// ISO-i-1. A list filters and sorts on the per-reader value.
func TestRollups_PathPerPrincipal(t *testing.T) {
	app, d := newRollupPathApp(t, 2, memstore.New())
	tests := []struct {
		user, query string
		want        string
	}{
		{"PERS-A", "sort=title", "NE-0 0 false; NE-1 3 true; NE-2 3 true"},
		{"PERS-B", "sort=title", "NE-0 0 false; NE-1 2 true; NE-2 2 true"},
		{"PERS-A", "filter[maatregel_aantal]=3&sort=title", "NE-1 3 true; NE-2 3 true"},
		{"PERS-B", "filter[maatregel_aantal]=3", ""},
		{"PERS-B", "filter[maatregel_aantal][gte]=1&sort=-title", "NE-2 2 true; NE-1 2 true"},
		{"PERS-A", "sort=-maatregel_aantal,title", "NE-1 3 true; NE-2 3 true; NE-0 0 false"},
	}
	for _, tc := range tests {
		t.Run(tc.user+" "+tc.query, func(t *testing.T) {
			resp, rec := listEntitiesAs(principalCtx(tc.user), t, app, d, "eis", "eiss", tc.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("list = %d %s", rec.Code, rec.Body)
			}
			rows := make([]string, 0, len(resp.Data))
			for _, row := range resp.Data {
				rows = append(rows, fmt.Sprintf("%s %v %v", row.ID, row.Properties["maatregel_aantal"], row.Properties["gedekt"]))
			}
			if got := strings.Join(rows, "; "); got != tc.want {
				t.Errorf("rows = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestQueryBudget_ListPathRollupsAreSizeIndependent pins that the path
// roll-up columns of a list page cost the same number of store reads at 10
// and 50 rows. memstore computes each in Go from two batched reads per hop;
// pgstore and sqlitestore answer each column in one statement.
func TestQueryBudget_ListPathRollupsAreSizeIndependent(t *testing.T) {
	extra := make([]int, 0, 2)
	var detail string
	for _, n := range []int{10, 50} {
		counting := storetest.NewCounting(memstore.New())
		app, d := newRollupPathApp(t, n, counting)
		ctx := principalCtx("PERS-B")
		reads := func(q string) int {
			counting.Reset()
			resp, rec := listEntitiesAs(ctx, t, app, d, "eis", "eiss", "per_page=100&"+q)
			if rec.Code != http.StatusOK || len(resp.Data) != n+1 {
				t.Fatalf("list %s: %d, %d rows (%s)", q, rec.Code, len(resp.Data), rec.Body)
			}
			return counting.Reads()
		}
		without := reads("rollups=false")
		with := reads("")
		detail = counting.String()
		extra = append(extra, with-without)
	}
	if extra[0] != extra[1] {
		t.Errorf("path roll-up reads grow with page size: %d extra at 10 rows, %d at 50 (%s)",
			extra[0], extra[1], detail)
	}
	const pinned = 8 // two columns of two hops; memstore folds each hop from two batched reads
	if extra[1] != pinned {
		t.Errorf("path roll-up reads = %d extra, pinned %d (%s) — update the pin if the change is deliberate",
			extra[1], pinned, detail)
	}
}
