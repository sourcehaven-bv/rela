package appbuild

import (
	"math"
	"testing"
)

// TestPageRange pins the paging of the flow lists, including caller
// values whose product would overflow.
func TestPageRange(t *testing.T) {
	tests := []struct {
		name           string
		n, page, per   int
		wantLo, wantHi int
	}{
		{"first page", 120, 1, 50, 0, 50},
		{"last partial page", 120, 3, 50, 100, 120},
		{"past the end", 120, 4, 50, 120, 120},
		{"defaults", 120, 0, 0, 0, 50},
		{"negative page", 120, -5, 10, 0, 10},
		{"empty list", 0, 1, 50, 0, 0},
		{"huge page", 120, math.MaxInt, 50, 120, 120},
		{"huge page and size", 120, math.MaxInt, math.MaxInt, 120, 120},
		{"huge size", 120, 1, math.MaxInt, 0, 120},
		{"huge size, second page", 120, 2, math.MaxInt, 120, 120},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lo, hi := pageRange(tc.n, tc.page, tc.per)
			if lo != tc.wantLo || hi != tc.wantHi {
				t.Errorf("pageRange(%d, %d, %d) = [%d, %d), want [%d, %d)", tc.n, tc.page, tc.per, lo, hi,
					tc.wantLo, tc.wantHi)
			}
		})
	}
}
