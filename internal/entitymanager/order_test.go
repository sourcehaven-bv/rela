package entitymanager

import (
	"math"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestMidpointOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		a, b      float64
		wantValue float64
		wantOK    bool
	}{
		{"simple integer gap", 1, 2, 1.5, true},
		{"non-integer endpoints", 1.25, 1.75, 1.5, true},
		{"large gap", 0, 1000, 500, true},
		{"negative range", -10, -5, -7.5, true},
		{"zero crossing", -1, 1, 0, true},
		{"identical values", 1, 1, 0, false},
		{"reversed", 2, 1, 0, false},
		{"gap below threshold", 1, 1 + OrderCollapseThreshold/2, 0, false},
		{"NaN a", math.NaN(), 1, 0, false},
		{"NaN b", 1, math.NaN(), 0, false},
		{"Inf b", 0, math.Inf(1), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := MidpointOrder(tt.a, tt.b)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (value=%v)", ok, tt.wantOK, v)
			}
			if ok && v != tt.wantValue {
				t.Errorf("value = %v, want %v", v, tt.wantValue)
			}
			if ok && (v <= tt.a || v >= tt.b) {
				t.Errorf("midpoint %v not strictly between %v and %v", v, tt.a, tt.b)
			}
		})
	}
}

func TestAppendOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		existing []float64
		want     float64
	}{
		{"empty", nil, 1.0},
		{"single", []float64{5}, 6.0},
		{"increasing", []float64{1, 2, 3}, 4.0},
		{"unsorted", []float64{2, 0.5, 3, 1.25}, 4.0},
		{"with NaN ignored", []float64{1, math.NaN(), 2}, 3.0},
		{"all NaN", []float64{math.NaN(), math.NaN()}, 1.0},
		{"negatives", []float64{-3, -10, -1}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendOrder(tt.existing); got != tt.want {
				t.Errorf("AppendOrder(%v) = %v, want %v", tt.existing, got, tt.want)
			}
		})
	}
}

func TestPrependOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		existing []float64
		want     float64
	}{
		{"empty", nil, 1.0},
		{"single", []float64{5}, 4.0},
		{"unsorted", []float64{3, 2, 1}, 0.0},
		{"with NaN ignored", []float64{2, math.NaN(), 3}, 1.0},
		{"negatives", []float64{-3, -10, -1}, -11.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrependOrder(tt.existing); got != tt.want {
				t.Errorf("PrependOrder(%v) = %v, want %v", tt.existing, got, tt.want)
			}
		})
	}
}

func TestNeedsRenumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		sorted []float64
		want   bool
	}{
		{"empty", nil, false},
		{"single", []float64{1}, false},
		{"safe gaps", []float64{1, 2, 3, 4}, false},
		{"tight but safe", []float64{1, 1.0001, 1.0002}, false},
		{"collapsed", []float64{1, 1 + OrderCollapseThreshold/2, 2}, true},
		{"exact duplicates", []float64{1, 1, 2}, true},
		{"non-finite skipped", []float64{1, math.NaN(), 1.5}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsRenumber(tt.sorted); got != tt.want {
				t.Errorf("NeedsRenumber(%v) = %v, want %v", tt.sorted, got, tt.want)
			}
		})
	}
}

// TestMidpoint_RepeatedInsertsTriggerCollapse simulates the worst-case
// scenario where every insert lands at the same position. After enough
// inserts, MidpointOrder must report collapse so the caller can renumber.
func TestMidpoint_RepeatedInsertsTriggerCollapse(t *testing.T) {
	t.Parallel()
	low, high := 1.0, 2.0
	const maxIters = 1000
	for range maxIters {
		v, ok := MidpointOrder(low, high)
		if !ok {
			// Collapse fired — that's the success case.
			return
		}
		// Always insert in the lower half to chase the collapse fastest.
		high = v
	}
	t.Fatalf("expected collapse within %d iterations; reached high=%v low=%v gap=%v", maxIters, high, low, high-low)
}

func TestSortRelations_StableMissingLast(t *testing.T) {
	t.Parallel()
	mkRel := func(id string, order interface{}) entity.Relation {
		props := map[string]interface{}{}
		if order != nil {
			props["_order_out"] = order
		}
		return entity.Relation{From: "src", Type: "has-step", To: id, Properties: props}
	}
	tests := []struct {
		name string
		in   []entity.Relation
		want []string // expected To order after sort
	}{
		{
			name: "ascending",
			in: []entity.Relation{
				mkRel("c", 3.0),
				mkRel("a", 1.0),
				mkRel("b", 2.0),
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "missing sorted last, stable among themselves",
			in: []entity.Relation{
				mkRel("missing1", nil),
				mkRel("a", 1.0),
				mkRel("missing2", nil),
				mkRel("b", 2.0),
			},
			want: []string{"a", "b", "missing1", "missing2"},
		},
		{
			name: "duplicate values, stable",
			in: []entity.Relation{
				mkRel("a", 1.0),
				mkRel("b", 1.0),
				mkRel("c", 1.0),
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "integer values accepted",
			in: []entity.Relation{
				mkRel("a", 3),
				mkRel("b", 1),
				mkRel("c", 2),
			},
			want: []string{"b", "c", "a"},
		},
		{
			name: "NaN treated as missing",
			in: []entity.Relation{
				mkRel("a", math.NaN()),
				mkRel("b", 1.0),
				mkRel("c", math.NaN()),
			},
			want: []string{"b", "a", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SortRelations(tt.in, "_order_out")
			gotIDs := make([]string, len(got))
			for i, r := range got {
				gotIDs[i] = r.To
			}
			if !slicesEqual(gotIDs, tt.want) {
				t.Errorf("got %v, want %v", gotIDs, tt.want)
			}
		})
	}
}

func TestSortRelations_EmptyPropertyName(t *testing.T) {
	t.Parallel()
	in := []entity.Relation{
		{To: "a", Properties: map[string]interface{}{"_order_out": 3.0}},
		{To: "b", Properties: map[string]interface{}{"_order_out": 1.0}},
	}
	got := SortRelations(in, "")
	if len(got) != 2 || got[0].To != "a" || got[1].To != "b" {
		t.Errorf("empty prop should not reorder; got %v", got)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
