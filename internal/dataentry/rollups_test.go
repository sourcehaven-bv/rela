package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Roll-up properties (TKT-6I2OPG) on the data-entry read surfaces.

const rollupWireMeta = `version: "1.0"
types:
  testresultaat:
    values: [geslaagd, gefaald]
entities:
  maatregel:
    label: Maatregel
    id_prefix: "MAAT-"
    properties:
      title: {type: string}
      bewijs_aantal:
        type: integer
        rollup: { via: evidencedBy, count: true, where: { status: geldig } }
      laatste_testresultaat:
        type: testresultaat
        rollup: { via: testedBy, pick: resultaat, latest_by: datum }
  bewijs:
    label: Bewijs
    id_prefix: "BEW-"
    properties:
      title: {type: string}
      status: {type: enum, values: [geldig, verlopen]}
  test:
    label: Test
    id_prefix: "TST-"
    properties:
      title: {type: string}
      resultaat: {type: testresultaat}
      datum: {type: date}
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      title: {type: string}
relations:
  evidences:
    from: [bewijs]
    to: [maatregel]
    inverse: evidencedBy
  tests:
    from: [test]
    to: [maatregel]
    inverse: testedBy
  shared-with:
    from: [person]
    to: [bewijs]
`

func rollupWireConfig() *Config {
	return &Config{
		App: dataentryconfig.AppConfig{Name: "rollups"},
		Lists: map[string]dataentryconfig.List{
			"maatregelen": {EntityType: "maatregel", Columns: []dataentryconfig.ListColumn{
				{Property: "title"}, {Property: "bewijs_aantal"},
			}},
		},
		Views: map[string]dataentryconfig.ViewConfig{
			"bewijs": {
				Title: "Bewijs",
				Entry: dataentryconfig.ViewEntry{Type: "bewijs"},
				Traverse: []dataentryconfig.ViewTraverse{
					{From: "entry", Follow: "evidences", CollectAs: "maatregelen"},
				},
				Sections: []dataentryconfig.ViewSection{{
					Heading: "Maatregelen", Source: "maatregelen", Display: "table",
					Columns: []dataentryconfig.ListColumn{{Property: "title"}, {Property: "bewijs_aantal"}},
				}},
			},
			"maatregel": {
				Title: "Maatregel",
				Entry: dataentryconfig.ViewEntry{Type: "maatregel"},
				Traverse: []dataentryconfig.ViewTraverse{
					{From: "entry", FollowIncoming: "tests", CollectAs: "tests"},
				},
				Sections: []dataentryconfig.ViewSection{{Heading: "Tests", Source: "tests", Display: "list"}},
			},
		},
		Forms:      map[string]dataentryconfig.Form{},
		Kanbans:    map[string]dataentryconfig.Kanban{},
		Navigation: []dataentryconfig.NavigationEntry{},
	}
}

// rollupWirePolicy gives three principals different reads:
//   - PERS-A reads everything.
//   - PERS-B reads maatregel and test, cannot see test.resultaat, and reads
//     only the bewijs rows shared with it (BEW-4 is not).
//   - PERS-C reads everything but cannot see bewijs.status.
//   - PERS-D reads everything but cannot see maatregel.bewijs_aantal.
func rollupWirePolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all": {Read: []string{"*"}},
			"base": {
				Read: []string{"maatregel", "test", "person"},
				Visible: map[string][]acl.FieldGrant{
					"test": {{Field: "title"}, {Field: "datum"}},
				},
			},
			"bewijslezer": {Read: []string{"bewijs"}},
			"statusblind": {
				Read:    []string{"*"},
				Visible: map[string][]acl.FieldGrant{"bewijs": {{Field: "title"}}},
			},
			"countblind": {
				Read: []string{"*"},
				Visible: map[string][]acl.FieldGrant{
					"maatregel": {{Field: "title"}, {Field: "laatste_testresultaat"}},
				},
			},
		},
		Assignments: map[string]string{
			"PERS-A": "all", "PERS-B": "base", "PERS-C": "statusblind", "PERS-D": "countblind",
		},
		RoleRelations: map[string]acl.RoleRelationDef{"shared-with": {Confers: "bewijslezer"}},
	}
}

// seedRollupWire writes n maatregelen. Each has the same related rows,
// so every one rolls up to the same values:
//   - bewijs geldig, geldig, verlopen (shared with PERS-B) and geldig (not).
//   - tests geslaagd on 2026-01-01 and gefaald on 2026-02-01.
func seedRollupWire(t *testing.T, st store.Store, n int) {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mk := func(id, typ string, props map[string]any) {
		e := entity.New(id, typ)
		maps.Copy(e.Properties, props)
		must(st.CreateEntity(ctx, e))
	}
	rel := func(from, typ, to string) {
		_, err := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil)
		must(err)
	}
	for _, p := range []string{"PERS-A", "PERS-B", "PERS-C", "PERS-D"} {
		mk(p, "person", map[string]any{"title": p})
	}
	for i := 1; i <= n; i++ {
		m := fmt.Sprintf("MAAT-%d", i)
		mk(m, "maatregel", map[string]any{"title": m, "bewijs_aantal": 99})
		for j, b := range []struct {
			status string
			shared bool
		}{{"geldig", true}, {"geldig", true}, {"verlopen", true}, {"geldig", false}} {
			id := fmt.Sprintf("BEW-%d-%d", i, j+1)
			mk(id, "bewijs", map[string]any{"title": id, "status": b.status})
			rel(id, "evidences", m)
			if b.shared {
				rel("PERS-B", "shared-with", id)
			}
		}
		for j, tr := range []struct{ resultaat, datum string }{
			{"geslaagd", "2026-01-01"}, {"gefaald", "2026-02-01"},
		} {
			id := fmt.Sprintf("TST-%d-%d", i, j+1)
			mk(id, "test", map[string]any{"title": id, "resultaat": tr.resultaat, "datum": tr.datum})
			rel(id, "tests", m)
		}
	}
}

// newRollupWireApp seeds n maatregelen into base, then assembles the app
// with the real field resolver over rollupWirePolicy.
func newRollupWireApp(t *testing.T, n int, base store.Store) (*App, *acl.Declarative) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(rollupWireMeta))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	seedRollupWire(t, base, n)
	app := newAppFromParts(rollupWireConfig(), meta, newFixture(), appbuildtest.WithStore(base))
	d := mustNewACL(t, rollupWirePolicy(), app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", meta, app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	return app, d
}

// rollupWant is what one principal sees on every maatregel. A nil field is
// an absent property.
type rollupWant struct {
	count  any
	latest any
}

var rollupWants = map[string]rollupWant{
	// Three geldig bewijs; the latest test is gefaald.
	"PERS-A": {count: float64(3), latest: "gefaald"},
	// BEW-x-4 is hidden, so it is not counted. resultaat is hidden, so the
	// latest test has no value to show and the roll-up is absent.
	"PERS-B": {count: float64(2), latest: nil},
	// status is hidden, so no bewijs row can be judged geldig.
	"PERS-C": {count: float64(0), latest: "gefaald"},
	// The roll-up property itself is hidden, so it is never served.
	"PERS-D": {count: nil, latest: "gefaald"},
}

func assertRollups(t *testing.T, where string, props map[string]any, want rollupWant) {
	t.Helper()
	if got, has := props["bewijs_aantal"]; got != want.count || (want.count == nil && has) {
		t.Errorf("%s: bewijs_aantal = %v, want %v", where, got, want.count)
	}
	got, has := props["laatste_testresultaat"]
	switch {
	case want.latest == nil && has:
		t.Errorf("%s: laatste_testresultaat = %v, want absent", where, got)
	case want.latest != nil && got != want.latest:
		t.Errorf("%s: laatste_testresultaat = %v, want %v", where, got, want.latest)
	}
}

func decodeProps(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var e struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("decode: %v (%s)", err, body)
	}
	return e.Properties
}

// sectionCell returns the first value of the cell at column col of the
// table-section row for entity id.
func sectionCell(t *testing.T, body []byte, id string, col int) (string, bool) {
	t.Helper()
	var view v1.ViewResponse
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode view: %v", err)
	}
	for _, s := range view.Sections {
		for _, row := range s.Rows {
			if row.EntityID == id && col < len(row.Cells) && len(row.Cells[col].Values) > 0 {
				return row.Cells[col].Values[0], true
			}
		}
	}
	return "", false
}

// TestRollups_ReadSurfacesPerPrincipal pins acceptance criteria 2 and 3:
// the entity GET, a list column and a view section serve the roll-up the
// caller's read rights give. A hidden related row is never counted, a row
// whose where property is hidden is excluded, and a hidden pick leaves the
// value absent. The stored bewijs_aantal (99) is never served.
func TestRollups_ReadSurfacesPerPrincipal(t *testing.T) {
	app, d := newRollupWireApp(t, 2, memstore.New())
	for user, want := range rollupWants {
		t.Run(user, func(t *testing.T) {
			ctx := principalCtx(user)

			rec := getEntityAs(ctx, t, app, d, "maatregel", "maatregels", "MAAT-1", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET = %d %s", rec.Code, rec.Body)
			}
			assertRollups(t, "GET", decodeProps(t, rec.Body.Bytes()), want)

			for _, q := range []string{"", "rollups=1", "list_id=maatregelen"} {
				resp, lrec := listEntitiesAs(ctx, t, app, d, "maatregel", "maatregels", q)
				if lrec.Code != http.StatusOK {
					t.Fatalf("list %s = %d %s", q, lrec.Code, lrec.Body)
				}
				if len(resp.Data) != 2 {
					t.Fatalf("list %s: %d rows, want 2", q, len(resp.Data))
				}
				for _, row := range resp.Data {
					assertRollups(t, "list "+q+" "+row.ID, row.Properties, want)
				}
			}
			resp, _ := listEntitiesAs(ctx, t, app, d, "maatregel", "maatregels", "rollups=false")
			for _, row := range resp.Data {
				if _, has := row.Properties["bewijs_aantal"]; has {
					t.Errorf("list with rollups=false carries bewijs_aantal on %s: %v", row.ID, row.Properties)
				}
			}

			rec = viewsAs(ctx, t, app, d, "maatregel", "MAAT-1")
			if rec.Code != http.StatusOK {
				t.Fatalf("view maatregel = %d %s", rec.Code, rec.Body)
			}
			var view map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			entry, _ := view["entry"].(map[string]any)
			entryProps, _ := entry["properties"].(map[string]any)
			assertRollups(t, "view entry", entryProps, want)

			rec = viewsAs(ctx, t, app, d, "bewijs", "BEW-1-1")
			if rec.Code != http.StatusOK {
				t.Fatalf("view bewijs = %d %s", rec.Code, rec.Body)
			}
			got, ok := sectionCell(t, rec.Body.Bytes(), "MAAT-1", 1)
			if want.count == nil {
				if ok {
					t.Errorf("view section bewijs_aantal = %q, want no value", got)
				}
				return
			}
			if !ok {
				t.Fatalf("view section has no MAAT-1 row: %s", rec.Body)
			}
			if want := fmt.Sprint(want.count); got != want {
				t.Errorf("view section bewijs_aantal = %q, want %q", got, want)
			}
		})
	}
}

// newRollupSortApp seeds three maatregelen whose roll-ups differ:
//   - MAAT-1: one geldig bewijs (shared with PERS-B); latest test gefaald.
//   - MAAT-2: three geldig bewijs, two shared with PERS-B; latest test
//     geslaagd.
//   - MAAT-3: no bewijs and no test, and a stored bewijs_aantal of 99 that
//     must never count.
func newRollupSortApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(rollupWireMeta))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	st := memstore.New()
	ctx := context.Background()
	mk := func(id, typ string, props map[string]any) {
		e := entity.New(id, typ)
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
	for _, p := range []string{"PERS-A", "PERS-B", "PERS-D"} {
		mk(p, "person", map[string]any{"title": p})
	}
	mk("MAAT-1", "maatregel", map[string]any{"title": "MAAT-1"})
	mk("MAAT-2", "maatregel", map[string]any{"title": "MAAT-2"})
	mk("MAAT-3", "maatregel", map[string]any{"title": "MAAT-3", "bewijs_aantal": 99})
	for id, b := range map[string]struct {
		to     string
		shared bool
	}{
		"BEW-1": {"MAAT-1", true}, "BEW-2": {"MAAT-2", true}, "BEW-3": {"MAAT-2", true}, "BEW-4": {"MAAT-2", false},
	} {
		mk(id, "bewijs", map[string]any{"title": id, "status": "geldig"})
		rel(id, "evidences", b.to)
		if b.shared {
			rel("PERS-B", "shared-with", id)
		}
	}
	for id, tr := range map[string]struct{ to, resultaat, datum string }{
		"TST-1": {"MAAT-1", "geslaagd", "2026-01-01"}, "TST-2": {"MAAT-1", "gefaald", "2026-02-01"},
		"TST-3": {"MAAT-2", "geslaagd", "2026-03-01"},
	} {
		mk(id, "test", map[string]any{"title": id, "resultaat": tr.resultaat, "datum": tr.datum})
		rel(id, "tests", tr.to)
	}
	app := newAppFromParts(rollupWireConfig(), meta, newFixture(), appbuildtest.WithStore(st))
	d := mustNewACL(t, rollupWirePolicy(), app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", meta, app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	return app, d
}

// TestRollups_ListFilterAndSort pins that a list filters and sorts on a
// roll-up as each reader computes it: PERS-B counts only the bewijs shared
// with it, and a value stored under the roll-up name never matches.
func TestRollups_ListFilterAndSort(t *testing.T) {
	app, d := newRollupSortApp(t)
	tests := []struct {
		user, query string
		want        []string
	}{
		{"PERS-A", "sort=-bewijs_aantal", []string{"MAAT-2", "MAAT-1", "MAAT-3"}},
		{"PERS-A", "sort=bewijs_aantal", []string{"MAAT-3", "MAAT-1", "MAAT-2"}},
		{"PERS-A", "filter[bewijs_aantal][gte]=1&sort=title", []string{"MAAT-1", "MAAT-2"}},
		{"PERS-A", "filter[bewijs_aantal]=2", nil},
		{"PERS-A", "filter[bewijs_aantal]=99", nil},
		{"PERS-B", "filter[bewijs_aantal]=2", []string{"MAAT-2"}},
		{"PERS-A", "filter[laatste_testresultaat]=gefaald", []string{"MAAT-1"}},
		{"PERS-A", "filter[bewijs_aantal][gte]=1&sort=-laatste_testresultaat", []string{"MAAT-1", "MAAT-2"}},
		{"PERS-A", "sort=-bewijs_aantal&per_page=1&page=2", []string{"MAAT-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.user+" "+tc.query, func(t *testing.T) {
			resp, rec := listEntitiesAs(principalCtx(tc.user), t, app, d, "maatregel", "maatregels", tc.query)
			if rec.Code != http.StatusOK {
				t.Fatalf("list = %d %s", rec.Code, rec.Body)
			}
			var got []string
			for _, row := range resp.Data {
				got = append(got, row.ID)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRollups_ListQueryOnHiddenRollupRefused pins that a reader who cannot
// see a roll-up cannot filter or sort on it: the field gate judges a
// roll-up as any other property.
func TestRollups_ListQueryOnHiddenRollupRefused(t *testing.T) {
	app, d := newRollupSortApp(t)
	for _, q := range []string{"filter[bewijs_aantal]=1", "sort=bewijs_aantal"} {
		_, rec := listEntitiesAs(principalCtx("PERS-D"), t, app, d, "maatregel", "maatregels", q)
		if rec.Code < 400 {
			t.Errorf("list ?%s = %d, want a refusal (%s)", q, rec.Code, rec.Body)
		}
	}
}

// TestRollups_WriteRefusedAndResponseCarriesValue pins that a PATCH naming
// a roll-up is refused, and that a write response serves the roll-ups as
// the GET does.
func TestRollups_WriteRefusedAndResponseCarriesValue(t *testing.T) {
	app, _ := newRollupWireApp(t, 1, memstore.New())
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"*"}, Update: []string{"maatregel"}}},
		Assignments: map[string]string{"PERS-A": "editor"},
	}, app.store)
	setTestACL(t, app, d)
	res, err := ResolverFromProfile("", app.State().Meta, app.store, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	ctx := principalCtx("PERS-A")

	rec := patchEntityAs(ctx, t, app, d, "maatregel", "maatregels", "MAAT-1",
		`{"properties":{"bewijs_aantal":7}}`, nil)
	if rec.Code < 400 {
		t.Errorf("PATCH of a roll-up = %d, want a refusal (%s)", rec.Code, rec.Body)
	}
	rec = patchEntityAs(ctx, t, app, d, "maatregel", "maatregels", "MAAT-1",
		`{"properties":{"title":"renamed"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	assertRollups(t, "PATCH response", decodeProps(t, rec.Body.Bytes()), rollupWants["PERS-A"])
}

// TestQueryBudget_ListRollupsAreSizeIndependent pins acceptance criterion
// 4: the roll-up columns of a list page cost the same number of store reads
// at 10 and 50 rows. On a store with a native roll-up (pgstore,
// sqlitestore) that is one Rollup call per column; memstore computes it in
// Go over a constant number of batched reads.
func TestQueryBudget_ListRollupsAreSizeIndependent(t *testing.T) {
	// measure returns the cost of a list page of n rows with roll-ups and
	// what the roll-ups add to it, in a store that also holds pad bewijs
	// rows no maatregel has.
	measure := func(n, pad int) (with, added storetest.Cost, detail string) {
		counting := storetest.NewCounting(memstore.New())
		app, d := newRollupWireApp(t, n, counting)
		for i := range pad {
			id := fmt.Sprintf("BEW-PAD-%d", i)
			e := entity.New(id, "bewijs")
			e.Properties["title"] = id
			e.Properties["status"] = "geldig"
			if err := counting.CreateEntity(context.Background(), e); err != nil {
				t.Fatal(err)
			}
			// An edge of a type the fixture uses, so a relation scan shows.
			if _, err := counting.CreateRelation(context.Background(),
				entity.RelationKey{From: "PERS-B", Type: "shared-with", To: id}, nil); err != nil {
				t.Fatal(err)
			}
		}
		ctx := principalCtx("PERS-B")
		reads := func(q string) storetest.Cost {
			counting.Reset()
			resp, rec := listEntitiesAs(ctx, t, app, d, "maatregel", "maatregels", "per_page=100&"+q)
			if rec.Code != http.StatusOK || len(resp.Data) != n {
				t.Fatalf("list %s: %d, %d rows (%s)", q, rec.Code, len(resp.Data), rec.Body)
			}
			return counting.Cost()
		}
		without := reads("rollups=false")
		with = reads("")
		return with, with.Sub(without), counting.String()
	}
	extra := make([]int, 0, 2)
	var detail string
	for _, n := range []int{10, 50} {
		_, c, d := measure(n, 0)
		extra = append(extra, c.Calls)
		detail = d
	}
	// The whole page with roll-ups, so a scan in the list itself shows too.
	storetest.AssertScaleFree(t, func(pad int) (storetest.Cost, string) {
		with, _, d := measure(10, pad)
		return with, d
	})
	if extra[0] != extra[1] {
		t.Errorf("roll-up reads grow with page size: %d extra at 10 rows, %d at 50 (%s)", extra[0], extra[1], detail)
	}
	const pinned = 4 // two columns; memstore folds each from two batched reads
	if extra[1] != pinned {
		t.Errorf("roll-up reads = %d extra, pinned %d (%s) — update the pin if the change is deliberate",
			extra[1], pinned, detail)
	}
}

// TestRollups_DashboardCards pins that a dashboard card reads a roll-up as
// a list row does: the table shows it, a breakdown groups on it, and a sum,
// avg or max card folds it, each per reader. PERS-D cannot see the
// roll-up, so its sum has no value and is partial.
func TestRollups_DashboardCards(t *testing.T) {
	app, d := newRollupSortApp(t)
	list := map[string]dataentryconfig.List{"all": {EntityType: "maatregel"}}
	installDashboardCards(app, list,
		dataentryconfig.DashboardCard{Title: "Total", List: "all", Display: "sum", Property: "bewijs_aantal"},
		dataentryconfig.DashboardCard{Title: "Average", List: "all", Display: "avg", Property: "bewijs_aantal"},
		dataentryconfig.DashboardCard{Title: "Most", List: "all", Display: "max", Property: "bewijs_aantal"},
		dataentryconfig.DashboardCard{Title: "Rows", List: "all", Display: "table",
			Sort:    []dataentryconfig.SortSpec{{Property: "bewijs_aantal", Direction: "desc"}},
			Columns: []dataentryconfig.ListColumn{{Property: "title"}, {Property: "bewijs_aantal"}}},
		dataentryconfig.DashboardCard{Title: "By result", List: "all", Display: "breakdown",
			GroupBy: "laatste_testresultaat"},
	)

	t.Run("PERS-A", func(t *testing.T) {
		ctx := gateCtxFor(principalCtx("PERS-A"), t, d)
		sum, _ := dashboardCardValueAs(ctx, t, app, 0)
		require.Equal(t, "bewijs_aantal", sum.Property)
		require.InDelta(t, 4, sum.Value, 0, "1 + 3 + 0; the stored 99 never counts")
		require.False(t, sum.Partial)
		avg, _ := dashboardCardValueAs(ctx, t, app, 1)
		require.Equal(t, "1.33", avg.Value)
		most, _ := dashboardCardValueAs(ctx, t, app, 2)
		require.InDelta(t, 3, most.Value, 0)
		table, _ := dashboardCardValueAs(ctx, t, app, 3)
		require.Len(t, table.Rows, 3)
		require.Equal(t, "MAAT-2", table.Rows[0].ID, "sorted by the roll-up, descending")
		require.InDelta(t, 3, table.Rows[0].Properties["bewijs_aantal"], 0)
		breakdown, _ := dashboardCardValueAs(ctx, t, app, 4)
		counts := map[string]int{}
		for _, g := range breakdown.Groups {
			counts[g.Value] = g.Count
		}
		require.Equal(t, map[string]int{"gefaald": 1, "geslaagd": 1, "": 1}, counts)
	})
	t.Run("PERS-B", func(t *testing.T) {
		ctx := gateCtxFor(principalCtx("PERS-B"), t, d)
		sum, _ := dashboardCardValueAs(ctx, t, app, 0)
		require.InDelta(t, 3, sum.Value, 0, "BEW-4 is not shared with PERS-B")
	})
	t.Run("PERS-D", func(t *testing.T) {
		ctx := gateCtxFor(principalCtx("PERS-D"), t, d)
		sum, body := dashboardCardValueAs(ctx, t, app, 0)
		require.Nil(t, sum.Value, body)
		require.True(t, sum.Partial)
		table, _ := dashboardCardValueAs(ctx, t, app, 3)
		for _, row := range table.Rows {
			require.NotContains(t, row.Properties, "bewijs_aantal")
		}
	})
}
