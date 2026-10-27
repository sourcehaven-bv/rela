package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"

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
	res, err := ResolverFromProfile("", meta, app.store, d)
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

			for _, q := range []string{"rollups=1", "list_id=maatregelen"} {
				resp, lrec := listEntitiesAs(ctx, t, app, d, "maatregel", "maatregels", q)
				if lrec.Code != http.StatusOK {
					t.Fatalf("list %s = %d %s", q, lrec.Code, lrec.Body)
				}
				if len(resp.Data) != 2 {
					t.Fatalf("list %s: %d rows, want 2", q, len(resp.Data))
				}
				for _, row := range resp.Data {
					w := want
					if q == "list_id=maatregelen" {
						w.latest = nil // not a column of the list
					}
					assertRollups(t, "list "+q+" "+row.ID, row.Properties, w)
				}
			}
			resp, _ := listEntitiesAs(ctx, t, app, d, "maatregel", "maatregels", "")
			for _, row := range resp.Data {
				if _, has := row.Properties["bewijs_aantal"]; has {
					t.Errorf("list without opt-in carries bewijs_aantal on %s: %v", row.ID, row.Properties)
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

// TestRollups_ListFilterAndSortRefused pins that a list cannot filter or
// sort on a roll-up: the value is not in the store, so the query cannot
// answer it.
func TestRollups_ListFilterAndSortRefused(t *testing.T) {
	app, d := newRollupWireApp(t, 1, memstore.New())
	for _, q := range []string{"filter[bewijs_aantal]=3", "sort=bewijs_aantal", "sort=-laatste_testresultaat"} {
		_, rec := listEntitiesAs(principalCtx("PERS-A"), t, app, d, "maatregel", "maatregels", q)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("list ?%s = %d, want 400 (%s)", q, rec.Code, rec.Body)
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
	res, err := ResolverFromProfile("", app.State().Meta, app.store, d)
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
	extra := make([]int, 0, 2)
	var detail string
	for _, n := range []int{10, 50} {
		counting := storetest.NewCounting(memstore.New())
		app, d := newRollupWireApp(t, n, counting)
		ctx := principalCtx("PERS-B")
		reads := func(q string) int {
			counting.Reset()
			resp, rec := listEntitiesAs(ctx, t, app, d, "maatregel", "maatregels", "per_page=100&"+q)
			if rec.Code != http.StatusOK || len(resp.Data) != n {
				t.Fatalf("list %s: %d, %d rows (%s)", q, rec.Code, len(resp.Data), rec.Body)
			}
			return counting.Reads()
		}
		without := reads("")
		with := reads("rollups=1")
		detail = counting.String()
		extra = append(extra, with-without)
	}
	if extra[0] != extra[1] {
		t.Errorf("roll-up reads grow with page size: %d extra at 10 rows, %d at 50 (%s)", extra[0], extra[1], detail)
	}
	const pinned = 4 // two columns; memstore folds each from two batched reads
	if extra[1] != pinned {
		t.Errorf("roll-up reads = %d extra, pinned %d (%s) — update the pin if the change is deliberate",
			extra[1], pinned, detail)
	}
}
