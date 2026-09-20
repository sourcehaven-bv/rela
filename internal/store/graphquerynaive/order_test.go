package graphquerynaive

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Order is the reference implementation of [store.GraphQuery.OrderBy] for
// every backend that does not push ordering into SQL — sqlite, fsstore and
// memstore all route GraphQuery through this package. So its ranking has to
// agree with the CASE expression pgstore emits, or the same view orders
// differently depending on which backend is deployed.
//
// The agreement is asserted here against the SQL semantics in prose, and
// against pgstore's generator in internal/store/pgstore/ordersql_test.go.
func TestOrder_RanksByDeclaredPosition(t *testing.T) {
	declared := []string{"todo", "done"}

	rows := func() []*entity.Entity {
		return []*entity.Entity{
			{ID: "A", Properties: map[string]any{"s": "done"}},  // rank 1
			{ID: "B", Properties: map[string]any{"s": "todo"}},  // rank 0
			{ID: "C", Properties: map[string]any{"s": "zeta"}},  // undeclared
			{ID: "D", Properties: map[string]any{"s": "alpha"}}, // undeclared
			{ID: "E", Properties: map[string]any{}},             // absent
		}
	}

	for _, tc := range []struct {
		name string
		desc bool
		want string
		why  string
	}{
		{
			name: "ascending", desc: false, want: "B,A,D,C,E",
			why: "declared order, then undeclared byte-wise, then absent last",
		},
		{
			name: "descending", desc: true, want: "E,C,D,A,B",
			why: "absent first (SQL's NULLS FIRST on DESC), then the mirror",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := rows()
			Order(got, []store.OrderSpec{{Property: "s", Values: declared, Descending: tc.desc}})
			if ids := joinIDs(got); ids != tc.want {
				t.Errorf("order = %s, want %s (%s)", ids, tc.want, tc.why)
			}
		})
	}
}

// A value listed twice ranks at its FIRST position, because SQL's
// `CASE x WHEN 'a' THEN 0 … WHEN 'a' THEN 2 END` takes the first matching arm.
// Nothing in schema.yaml rejects a duplicate, so this is reachable by typo and
// would otherwise sort differently here than in the database.
func TestOrder_DuplicateDeclaredValueUsesFirstPosition(t *testing.T) {
	rows := []*entity.Entity{
		{ID: "X", Properties: map[string]any{"s": "b"}},
		{ID: "Y", Properties: map[string]any{"s": "a"}},
	}
	Order(rows, []store.OrderSpec{{Property: "s", Values: []string{"a", "b", "a"}}})
	if got := joinIDs(rows); got != "Y,X" {
		t.Errorf("order = %s, want Y,X (the duplicate 'a' ranks 0, not 2)", got)
	}
}

// With no declared values a key compares byte-wise, which is the contract
// every backend held before ranking existed. Mixed case matters: a natural or
// case-insensitive order would put "apple" first.
func TestOrder_WithoutDeclaredValuesComparesByteWise(t *testing.T) {
	rows := []*entity.Entity{
		{ID: "A", Properties: map[string]any{"s": "apple"}},
		{ID: "B", Properties: map[string]any{"s": "Zebra"}},
		{ID: "C", Properties: map[string]any{"s": "item10"}},
		{ID: "D", Properties: map[string]any{"s": "item9"}},
	}
	Order(rows, []store.OrderSpec{{Property: "s"}})
	if got := joinIDs(rows); got != "B,A,C,D" {
		t.Errorf("order = %s, want B,A,C,D (byte-wise: uppercase first, item10 before item9)", got)
	}
}

// Ties break by id ascending in BOTH directions, matching
// `ORDER BY <key> DESC, id ASC`. Breaking ties by id descending would move
// page boundaries on every descending list.
func TestOrder_TiesBreakByIDAscendingInBothDirections(t *testing.T) {
	for _, desc := range []bool{false, true} {
		rows := []*entity.Entity{
			{ID: "TKT-3", Properties: map[string]any{"s": "todo"}},
			{ID: "TKT-1", Properties: map[string]any{"s": "todo"}},
			{ID: "TKT-2", Properties: map[string]any{"s": "todo"}},
		}
		Order(rows, []store.OrderSpec{{Property: "s", Values: []string{"todo"}, Descending: desc}})
		if got := joinIDs(rows); got != "TKT-1,TKT-2,TKT-3" {
			t.Errorf("tiebreak (desc=%v) = %s, want TKT-1,TKT-2,TKT-3", desc, got)
		}
	}
}

func joinIDs(rows []*entity.Entity) string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return strings.Join(out, ",")
}
