package filter

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestMatchDecimal(t *testing.T) {
	scalar := &metamodel.PropertyDef{Type: metamodel.PropertyTypeDecimal}
	list := &metamodel.PropertyDef{Type: metamodel.PropertyTypeDecimal, List: true}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		def     *metamodel.PropertyDef
		value   any
		filter  string
		want    bool
		wantErr bool
	}{
		{"equal ignores trailing zero", scalar, "12.50", "price=12.5", true, false},
		{"equal float value", scalar, 0.3, "price=0.30", true, false},
		{"not equal", scalar, "12.50", "price!=12.5", false, false},
		{"numeric not lexical less", scalar, "9", "price<10", true, false},
		{"less false", scalar, "10.01", "price<10", false, false},
		{"less equal boundary", scalar, "10.00", "price<=10", true, false},
		{"greater", scalar, "0.1", "price>0.09", true, false},
		{"greater equal boundary", scalar, "-1.5", "price>=-1.50", true, false},
		{"no float drift", scalar, "0.3", "price=0.30000000000000004", false, false},
		{"empty never orders", scalar, "", "price>0", false, false},
		{"empty equals empty", scalar, nil, "price=", true, false},
		{"regex refused", scalar, "1.5", "price~=1", false, true},
		{"invalid entity value", scalar, "abc", "price=5", false, true},
		{"invalid filter value", scalar, "5", "price=abc", false, true},
		{"list member by number", list, []any{"2.50", "3"}, "price=2.5", true, false},
		{"list not member", list, []any{"2.50", "3"}, "price!=2.5", false, false},
		{"list none equal", list, []any{"2.50", "3"}, "price!=4", true, false},
		{"list ordered any", list, []any{"2.50", "3"}, "price>2.9", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &entity.Entity{ID: "T-1", Type: "t", Properties: map[string]any{"price": tt.value}}
			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.filter, err)
			}
			got, err := Match(toRecord(e), f, tt.def, mm)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortDecimal(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeDecimal}
	entities := []*entity.Entity{
		{ID: "a", Properties: map[string]any{"price": "10"}},
		{ID: "b", Properties: map[string]any{"price": "9.99"}},
		{ID: "c", Properties: map[string]any{"price": 0.5}},
		{ID: "d", Properties: map[string]any{"price": "-2"}},
		{ID: "e"},
		{ID: "f", Properties: map[string]any{"price": "10.000001"}},
	}
	Sort(entities, testAccess, "price", propDef, nil, false)
	assertOrder(t, entities, "d", "c", "b", "a", "f", "e")

	Sort(entities, testAccess, "price", propDef, nil, true)
	assertOrder(t, entities, "f", "a", "b", "c", "d", "e")
}

// A column mixing an integer type and a decimal type sorts by number.
func TestSortMulti_MixedIntegerDecimal(t *testing.T) {
	defs := map[string]*metamodel.EntityDef{
		"whole": {Properties: map[string]metamodel.PropertyDef{"n": {Type: metamodel.PropertyTypeInteger}}},
		"frac":  {Properties: map[string]metamodel.PropertyDef{"n": {Type: metamodel.PropertyTypeDecimal}}},
	}
	entities := []*entity.Entity{
		{ID: "w10", Type: "whole", Properties: map[string]any{"n": 10}},
		{ID: "f9", Type: "frac", Properties: map[string]any{"n": "9.5"}},
		{ID: "w2", Type: "whole", Properties: map[string]any{"n": 2}},
		{ID: "f100", Type: "frac", Properties: map[string]any{"n": "100.25"}},
	}
	SortMulti(entities, testAccess, []SortSpec{{Property: "n"}}, defs, nil)
	assertOrder(t, entities, "w2", "f9", "w10", "f100")
}

func assertOrder(t *testing.T, entities []*entity.Entity, want ...string) {
	t.Helper()
	for i, e := range entities {
		if e.ID != want[i] {
			got := make([]string, len(entities))
			for j, x := range entities {
				got[j] = x.ID
			}
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
