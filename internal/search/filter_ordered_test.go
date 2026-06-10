package search

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestValidateFilters_RejectsOrderedOps(t *testing.T) {
	ordered := []FilterOp{FilterGt, FilterLt, FilterGte, FilterLte}
	for _, op := range ordered {
		err := ValidateFilters([]PropertyFilter{{Property: "count", Value: "9", Op: op}})
		if !errors.Is(err, ErrOrderedFilterUnsupported) {
			t.Errorf("op %v: expected ErrOrderedFilterUnsupported, got %v", op, err)
		}
	}
}

func TestValidateFilters_AllowsSupportedOps(t *testing.T) {
	supported := []FilterOp{FilterEq, FilterNe, FilterContains, FilterIn, FilterExists, FilterNotExists}
	for _, op := range supported {
		if err := ValidateFilters([]PropertyFilter{{Property: "status", Value: "open", Op: op}}); err != nil {
			t.Errorf("op %v: unexpected error %v", op, err)
		}
	}
}

// TestMatchFilters_OrderedOpIsNonMatch pins the defensive behavior: if
// an ordered op bypasses ValidateFilters and reaches MatchFilters, it is
// a non-match rather than a silent lexicographic comparison.
func TestMatchFilters_OrderedOpIsNonMatch(t *testing.T) {
	e := entity.New("REQ-1", "requirement")
	e.SetString("count", "10")
	if MatchFilters(e, []PropertyFilter{{Property: "count", Value: "9", Op: FilterGt}}) {
		t.Error("ordered op should not match (no silent lexicographic comparison)")
	}
}
