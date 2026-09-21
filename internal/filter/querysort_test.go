package filter

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The query-sort rule is store.GraphQuery.OrderBy's contract. These tests pin
// it directly rather than through a handler, because the same rule has to hold
// for every backend and the contract is what they are all verified against.

type qsRow struct {
	id    string
	props map[string]any
}

func qsAccess(r qsRow) Record {
	return Record{ID: r.id, Type: "ticket", Properties: r.props}
}

func qsDefs(props map[string]metamodel.PropertyDef) map[string]*metamodel.EntityDef {
	return map[string]*metamodel.EntityDef{"ticket": {Properties: props}}
}

func qsIDs(rows []qsRow) string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.id)
	}
	return strings.Join(out, ",")
}

func qsSort(t *testing.T, rows []qsRow, specs []SortSpec, defs map[string]*metamodel.EntityDef) string {
	t.Helper()
	qs := NewQuerySort(specs, defs, &metamodel.Metamodel{})
	QuerySortApply(qs, rows, qsAccess, specs)
	return qsIDs(rows)
}

func statusDefs() map[string]*metamodel.EntityDef {
	return qsDefs(map[string]metamodel.PropertyDef{
		"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"todo", "doing", "blocked", "done"}},
		"due":    {Type: metamodel.PropertyTypeDate},
		"title":  {Type: metamodel.PropertyTypeString},
	})
}

func rowsWithStatus(pairs ...[2]string) []qsRow {
	rows := make([]qsRow, 0, len(pairs))
	for _, p := range pairs {
		rows = append(rows, qsRow{id: p[0], props: map[string]any{"status": p[1]}})
	}
	return rows
}

// AC2: declared order, not alphabetical. Alphabetical would be
// blocked,doing,done,todo — a workflow enum read as a dictionary.
func TestQuerySort_EnumUsesDeclaredOrder(t *testing.T) {
	rows := rowsWithStatus(
		[2]string{"A", "done"}, [2]string{"B", "todo"},
		[2]string{"C", "blocked"}, [2]string{"D", "doing"},
	)
	if got, want := qsSort(t, rows, []SortSpec{{Property: "status"}}, statusDefs()), "B,D,C,A"; got != want {
		t.Errorf("declared order = %s, want %s", got, want)
	}
}

// A value the schema no longer declares sorts after every declared one, and
// byte-wise among its peers — matching the CASE's ELSE arm.
func TestQuerySort_UndeclaredEnumValuesSortLast(t *testing.T) {
	rows := rowsWithStatus(
		[2]string{"A", "zeta"}, [2]string{"B", "done"},
		[2]string{"C", "alpha"}, [2]string{"D", "todo"},
	)
	if got, want := qsSort(t, rows, []SortSpec{{Property: "status"}}, statusDefs()), "D,B,C,A"; got != want {
		t.Errorf("unknown-last = %s, want %s", got, want)
	}
}

// AC3. The `return !less` shape this replaces reported BOTH orderings for two
// equal keys, which is not a valid strict weak ordering: 13 equal keys came
// back reversed rather than stable.
func TestQuerySort_DescendingKeepsEqualKeysStable(t *testing.T) {
	var rows []qsRow
	var want []string
	for _, c := range "ABCDEFGHIJKLM" {
		rows = append(rows, qsRow{id: string(c), props: map[string]any{"status": "todo"}})
		want = append(want, string(c))
	}
	got := qsSort(t, rows, []SortSpec{{Property: "status", Direction: "desc"}}, statusDefs())
	if got != strings.Join(want, ",") {
		t.Errorf("equal keys under desc = %s, want %s (input order)", got, strings.Join(want, ","))
	}
}

// AC3, the consequence that actually corrupts a page: a descending primary key
// used to invert the secondary key it was supposed to leave alone.
func TestQuerySort_DescendingPrimaryKeepsSecondaryAscending(t *testing.T) {
	rows := []qsRow{
		{id: "r1", props: map[string]any{"status": "todo", "due": "2026-03-01"}},
		{id: "r2", props: map[string]any{"status": "todo", "due": "2026-01-01"}},
		{id: "r3", props: map[string]any{"status": "todo", "due": "2026-02-01"}},
	}
	specs := []SortSpec{{Property: "status", Direction: "desc"}, {Property: "due"}}
	if got, want := qsSort(t, rows, specs, statusDefs()), "r2,r3,r1"; got != want {
		t.Errorf("secondary key under desc primary = %s, want %s", got, want)
	}
}

// AC3. Ties break by id ASCENDING in both directions, matching
// `ORDER BY <key> DESC, id ASC`. Breaking ties by id desc would move page
// boundaries on every descending list.
func TestQuerySort_IDTiebreakIsAscendingInBothDirections(t *testing.T) {
	for _, dir := range []string{"asc", "desc"} {
		rows := rowsWithStatus(
			[2]string{"TKT-3", "todo"}, [2]string{"TKT-1", "todo"}, [2]string{"TKT-2", "todo"},
		)
		got := qsSort(t, rows, []SortSpec{{Property: "status", Direction: dir}}, statusDefs())
		if want := "TKT-1,TKT-2,TKT-3"; got != want {
			t.Errorf("tiebreak (%s) = %s, want %s", dir, got, want)
		}
	}
}

// AC8. SQL's default null placement, and the reason one index serves both
// directions: absent last ascending, first descending.
func TestQuerySort_NullPlacementFollowsDirection(t *testing.T) {
	cases := []struct{ dir, want string }{
		{"asc", "B,D,A,C"},
		{"desc", "A,C,D,B"}, // absent first, then ranks reversed: doing(1) before todo(0)
	}
	for _, tc := range cases {
		rows := []qsRow{
			{id: "A", props: map[string]any{}},                  // absent
			{id: "B", props: map[string]any{"status": "todo"}},  // rank 0
			{id: "C", props: map[string]any{"status": nil}},     // JSON null
			{id: "D", props: map[string]any{"status": "doing"}}, // rank 1
		}
		if got := qsSort(t, rows, []SortSpec{{Property: "status", Direction: tc.dir}}, statusDefs()); got != tc.want {
			t.Errorf("null placement (%s) = %s, want %s", tc.dir, got, tc.want)
		}
	}
}

// AC8, and the decision that strings follow SQL rather than the reverse.
// natsort would give apple,item9,item10,Zebra.
func TestQuerySort_StringsCompareByteWise(t *testing.T) {
	rows := []qsRow{
		{id: "A", props: map[string]any{"title": "Zebra"}},
		{id: "B", props: map[string]any{"title": "apple"}},
		{id: "C", props: map[string]any{"title": "item10"}},
		{id: "D", props: map[string]any{"title": "item9"}},
	}
	if got, want := qsSort(t, rows, []SortSpec{{Property: "title"}}, statusDefs()), "A,B,C,D"; got != want {
		t.Errorf("byte-wise strings = %s, want %s", got, want)
	}
}

// AC8. `->>` yields a text form for every JSON value, so a list, a number and a
// bool all sort as text. The previous comparator type-asserted .(string) and
// returned false for every pair, which silently did not sort at all.
func TestQuerySort_NonStringValuesSortByTextForm(t *testing.T) {
	rows := []qsRow{
		{id: "A", props: map[string]any{"tags": []any{"z"}}},
		{id: "B", props: map[string]any{"tags": []any{"a"}}},
		{id: "C", props: map[string]any{"tags": []any{"m"}}},
	}
	if got, want := qsSort(t, rows, []SortSpec{{Property: "tags"}}, statusDefs()), "B,C,A"; got != want {
		t.Errorf("list-valued sort = %s, want %s", got, want)
	}
}

// AC8. An undeclared property is still ordered by `->>` in SQL, so the Go path
// orders it too rather than leaving the rows untouched.
func TestQuerySort_UndeclaredPropertySortsByteWise(t *testing.T) {
	rows := []qsRow{
		{id: "A", props: map[string]any{"nope": "Zebra"}},
		{id: "B", props: map[string]any{"nope": "apple"}},
	}
	if got, want := qsSort(t, rows, []SortSpec{{Property: "nope"}}, statusDefs()), "A,B"; got != want {
		t.Errorf("undeclared property = %s, want %s", got, want)
	}
}

// sort=id is byte order, matching `ORDER BY id`. natsort would give
// TKT-2,TKT-9,TKT-10.
func TestQuerySort_ExplicitIDKeyIsByteOrder(t *testing.T) {
	rows := []qsRow{
		{id: "TKT-9", props: map[string]any{}},
		{id: "TKT-10", props: map[string]any{}},
		{id: "TKT-2", props: map[string]any{}},
	}
	if got, want := qsSort(t, rows, []SortSpec{{Property: "id"}}, statusDefs()), "TKT-10,TKT-2,TKT-9"; got != want {
		t.Errorf("sort=id = %s, want %s", got, want)
	}
}

// A property declaring different value orders on two types cannot rank a mixed
// result set; falling back to byte-wise beats picking whichever type was seen
// first, which would make the order depend on map iteration.
func TestQuerySort_ConflictingDeclaredOrdersFallBackToByteWise(t *testing.T) {
	defs := map[string]*metamodel.EntityDef{
		"ticket": {Properties: map[string]metamodel.PropertyDef{
			"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"todo", "done"}},
		}},
		"bug": {Properties: map[string]metamodel.PropertyDef{
			"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"done", "todo"}},
		}},
	}
	qs := NewQuerySort([]SortSpec{{Property: "status"}}, defs, &metamodel.Metamodel{})
	if rank := qs.Ranks("status"); rank != nil {
		t.Fatalf("conflicting declared orders should yield no rank, got %v", rank)
	}
}

// A custom type's `values:` ranks exactly like an inline enum — the Go side
// already did this, and the SQL side must match or the two diverge for the
// properties the Go side handles most quietly.
func TestQuerySort_CustomTypeValuesRank(t *testing.T) {
	meta := &metamodel.Metamodel{Types: map[string]metamodel.CustomType{
		"severity": {Values: []string{"critical", "minor"}},
	}}
	defs := qsDefs(map[string]metamodel.PropertyDef{"sev": {Type: "severity"}})
	specs := []SortSpec{{Property: "sev"}}
	rows := []qsRow{
		{id: "A", props: map[string]any{"sev": "minor"}},
		{id: "B", props: map[string]any{"sev": "critical"}},
	}
	qs := NewQuerySort(specs, defs, meta)
	QuerySortApply(qs, rows, qsAccess, specs)
	if got, want := qsIDs(rows), "B,A"; got != want {
		t.Errorf("custom type rank = %s, want %s", got, want)
	}
}

// A value listed twice is a schema typo nothing rejects, so the three
// rankings — this comparator, graphquerynaive, and SQL's CASE — have to agree
// on which position wins. SQL takes the FIRST matching arm (verified:
// `CASE 'a' WHEN 'a' THEN 0 WHEN 'b' THEN 1 WHEN 'a' THEN 2 END` = 0), so a
// last-wins index would sort a duplicated value differently in Go than in the
// database.
func TestQuerySort_DuplicateDeclaredValueRanksAtItsFirstPosition(t *testing.T) {
	defs := qsDefs(map[string]metamodel.PropertyDef{
		"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "wip", "open", "done"}},
	})
	specs := []SortSpec{{Property: "status"}}
	if got := NewQuerySort(specs, defs, &metamodel.Metamodel{}).Ranks("status")["open"]; got != 0 {
		t.Errorf("duplicated value ranked at %d, want 0 (its first position, as SQL's CASE does)", got)
	}

	rows := []qsRow{
		{id: "A", props: map[string]any{"status": "done"}},
		{id: "B", props: map[string]any{"status": "open"}},
		{id: "C", props: map[string]any{"status": "wip"}},
	}
	if got, want := qsSort(t, rows, specs, defs), "B,C,A"; got != want {
		t.Errorf("order = %s, want %s", got, want)
	}
}

// `sort:modified` is honored on the Go path. The search bar documents it and
// searchparser parses it, and search results are already materialized, so
// there is no pushed query for this to disagree with — the pushdown planner
// rejects the key independently because no entity type declares a property
// called "modified".
//
// Regression: the first version of this comparator only special-cased "id",
// so "modified" fell through to the property lookup, every row compared
// absent, and the whole result tied through to the id tiebreak.
func TestQuerySort_ModifiedOrdersByModificationTime(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	type row struct {
		id string
		at time.Time
	}
	acc := func(r row) Record {
		return Record{ID: r.id, Type: "ticket", Properties: map[string]any{}, ModifiedAt: r.at}
	}
	run := func(dir string) string {
		rows := []row{
			{"A", base},
			{"B", base.Add(48 * time.Hour)},
			{"C", base.Add(24 * time.Hour)},
			{"D", time.Time{}}, // never recorded: sorts as absent
		}
		specs := []SortSpec{{Property: "modified", Direction: dir}}
		QuerySortApply(NewQuerySort(specs, statusDefs(), &metamodel.Metamodel{}), rows, acc, specs)
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.id)
		}
		return strings.Join(out, ",")
	}

	if got, want := run("asc"), "A,C,B,D"; got != want {
		t.Errorf("sort:modified asc = %s, want %s", got, want)
	}
	if got, want := run("desc"), "D,B,C,A"; got != want {
		t.Errorf("sort:modified desc = %s, want %s", got, want)
	}
}
