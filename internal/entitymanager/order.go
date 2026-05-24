package entitymanager

import (
	"math"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// OrderCollapseThreshold is the minimum gap between adjacent order values
// before the entity manager renumbers a side to dense integer ordinals.
// Picked well above IEEE-754 float64 precision-loss territory so the
// renumber fires defensively, not at the edge of correctness.
const OrderCollapseThreshold = 1e-9

// MidpointOrder returns a value strictly between a and b. If the gap is
// below OrderCollapseThreshold or the values are not in strict ascending
// order, returns (0, false) so the caller can decide to renumber.
func MidpointOrder(a, b float64) (float64, bool) {
	if !isFiniteFloat(a) || !isFiniteFloat(b) {
		return 0, false
	}
	if b-a < OrderCollapseThreshold {
		return 0, false
	}
	return a + (b-a)/2, true
}

// AppendOrder returns a value strictly greater than every finite value in
// existing. When existing is empty (or has no finite values), returns 1.0.
func AppendOrder(existing []float64) float64 {
	maxV := math.Inf(-1)
	for _, v := range existing {
		if isFiniteFloat(v) && v > maxV {
			maxV = v
		}
	}
	if math.IsInf(maxV, -1) {
		return 1.0
	}
	return maxV + 1.0
}

// PrependOrder returns a value strictly less than every finite value in
// existing. When existing is empty (or has no finite values), returns 1.0.
func PrependOrder(existing []float64) float64 {
	minV := math.Inf(1)
	for _, v := range existing {
		if isFiniteFloat(v) && v < minV {
			minV = v
		}
	}
	if math.IsInf(minV, 1) {
		return 1.0
	}
	return minV - 1.0
}

// NeedsRenumber reports whether a sorted list of order values has any
// adjacent gap below OrderCollapseThreshold. The input must already be
// sorted ascending; values are otherwise compared in slice order.
func NeedsRenumber(sorted []float64) bool {
	for i := 1; i < len(sorted); i++ {
		a, b := sorted[i-1], sorted[i]
		if !isFiniteFloat(a) || !isFiniteFloat(b) {
			continue
		}
		if b-a < OrderCollapseThreshold {
			return true
		}
	}
	return false
}

// SortRelations returns a copy of rels in stable sort order by the named
// numeric property ascending. Entries with a missing, non-numeric, or
// non-finite value sort after entries with a finite value; among themselves
// they preserve the original input order (stable). Ties on the value are
// also broken by original order.
//
// When prop is empty, returns a shallow copy of rels in input order.
func SortRelations(rels []entity.Relation, prop string) []entity.Relation {
	out := make([]entity.Relation, len(rels))
	copy(out, rels)
	if prop == "" {
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		vi, oki := orderValue(out[i], prop)
		vj, okj := orderValue(out[j], prop)
		switch {
		case oki && !okj:
			return true
		case !oki && okj:
			return false
		case !oki && !okj:
			return false
		}
		return vi < vj
	})
	return out
}

// orderValue extracts a finite float ordering value from a relation's
// properties using FiniteOrder semantics on the named property.
func orderValue(r entity.Relation, prop string) (float64, bool) {
	return FiniteOrder(r.Properties[prop])
}

// FiniteOrder is re-exported from metamodel for callers that already
// import entitymanager. The canonical implementation lives in metamodel
// so the analyzer (which can't depend on entitymanager) shares it.
func FiniteOrder(v interface{}) (float64, bool) {
	return metamodel.FiniteOrder(v)
}

func isFiniteFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
