package entitymanager

import (
	"fmt"
	"math"
	"slices"

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

// SortRelations returns a copy of rels sorted by the named order property
// with [metamodel.CompareOrderKeys]: finite values first, ascending, then
// edges without one; ties and missing values by the far endpoint's id, then
// the tail. The far endpoint is the target for [metamodel.OrderPropertyOut]
// and the source for [metamodel.OrderPropertyIn].
//
// When prop is empty, returns a shallow copy of rels in input order.
func SortRelations(rels []entity.Relation, prop string) []entity.Relation {
	out := make([]entity.Relation, len(rels))
	copy(out, rels)
	if prop == "" {
		return out
	}
	slices.SortStableFunc(out, func(a, b entity.Relation) int {
		return metamodel.CompareOrderKeys(OrderKeyOf(a, prop), OrderKeyOf(b, prop))
	})
	return out
}

// OrderKeyOf returns r's place on the order side prop names.
func OrderKeyOf(r entity.Relation, prop string) metamodel.OrderKey {
	peer := r.To
	if prop == metamodel.OrderPropertyIn {
		peer = r.From
	}
	return metamodel.OrderKey{Value: r.Properties[prop], Peer: peer, Tail: string(r.FromFace)}
}

// FiniteOrder is re-exported from metamodel for callers that already
// import entitymanager. The canonical implementation lives in metamodel
// so the analyzer (which can't depend on entitymanager) shares it.
func FiniteOrder(v any) (float64, bool) {
	return metamodel.FiniteOrder(v)
}

func isFiniteFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// PlaceOrder plans the writes that move the edge moved to pos among its
// siblings on the order side prop. siblings is every edge on that side the
// move may see and write, moved included, in any order. The result maps each edge to rewrite to its
// new value; it is empty when the move changes nothing.
//
// Usually only moved is rewritten, to a value between its new neighbors.
// When the edge above the new place has no value, or the gap between the
// two neighbors has collapsed, no value fits, so siblings is densified to
// 1..N in the new order. Edges without a value sort after all
// valued ones, so landing directly after the last valued edge needs no
// densify: prev+1 already puts it there.
func PlaceOrder(
	siblings []entity.Relation, moved entity.RelationKey, pos entity.OrderPosition, prop string,
) (map[entity.RelationKey]float64, error) {
	sorted := SortRelations(siblings, prop)
	from := slices.IndexFunc(sorted, func(r entity.Relation) bool { return r.Identity() == moved })
	if from < 0 {
		return nil, fmt.Errorf("%w: %s", ErrRelationNotFound, moved)
	}
	rest := slices.Delete(slices.Clone(sorted), from, from+1)
	to, err := insertionIndex(rest, from, moved, pos, prop)
	if err != nil {
		return nil, err
	}
	if to == from {
		return map[entity.RelationKey]float64{}, nil
	}
	final := slices.Insert(rest, to, sorted[from])

	value, ok := slotValue(final, to, prop)
	if ok {
		return map[entity.RelationKey]float64{moved: value}, nil
	}
	plan := make(map[entity.RelationKey]float64, len(final))
	for i, r := range final {
		want := float64(i + 1)
		if cur, has := FiniteOrder(r.Properties[prop]); has && cur == want {
			continue
		}
		plan[r.Identity()] = want
	}
	return plan, nil
}

// insertionIndex resolves pos to an index into rest, the order with the
// moved edge taken out; from is where the moved edge was.
func insertionIndex(
	rest []entity.Relation, from int, moved entity.RelationKey, pos entity.OrderPosition, prop string,
) (int, error) {
	set := 0
	for _, named := range []bool{pos.Before != "", pos.After != "", pos.Step != 0} {
		if named {
			set++
		}
	}
	if set != 1 {
		return 0, fmt.Errorf("%w: name exactly one of before, after and step", ErrInvalidOrderPosition)
	}
	if pos.Step != 0 {
		if pos.Step != -1 && pos.Step != 1 {
			return 0, fmt.Errorf("%w: step must be -1 or 1", ErrInvalidOrderPosition)
		}
		return min(max(from+pos.Step, 0), len(rest)), nil
	}
	ref, tail, anyTail, err := orderRef(pos, moved, prop)
	if err != nil {
		return 0, err
	}
	if ref == OrderKeyOf(entity.Relation{From: moved.From, To: moved.To}, prop).Peer &&
		(anyTail || tail == string(moved.FromFace)) {

		return 0, fmt.Errorf("%w: an edge cannot move relative to itself", ErrInvalidOrderPosition)
	}
	// rest is in order, so a ref naming no tail matches the peer's first
	// place: the one a list showing each peer once shows.
	i := slices.IndexFunc(rest, func(r entity.Relation) bool {
		k := OrderKeyOf(r, prop)
		return k.Peer == ref && (anyTail || k.Tail == tail)
	})
	if i < 0 {
		return 0, ErrOrderRefNotSibling
	}
	if pos.After != "" {
		i++
	}
	return i, nil
}

// orderRef reads the sibling a Before or After names. On the outgoing side
// it is a target id on the moved edge's own tail. On the incoming side it
// is a source address: `id@face` names that tail, and a bare id any tail
// (anyTail).
func orderRef(
	pos entity.OrderPosition, moved entity.RelationKey, prop string,
) (peer, tail string, anyTail bool, err error) {
	ref := pos.Before + pos.After
	if prop != metamodel.OrderPropertyIn {
		return ref, string(moved.FromFace), false, nil
	}
	addr, err := entity.ParseAddress(ref)
	if err != nil {
		return "", "", false, fmt.Errorf("%w: %w", ErrInvalidOrderPosition, err)
	}
	if named, ok := addr.Named(); ok {
		return named.ID, string(named.Face), false, nil
	}
	return addr.ID(), "", true, nil
}

// slotValue returns a value that puts final[at] between its neighbors
// without touching them, or false when none exists.
func slotValue(final []entity.Relation, at int, prop string) (float64, bool) {
	var prev, next *float64
	if at > 0 {
		v, ok := FiniteOrder(final[at-1].Properties[prop])
		if !ok {
			return 0, false
		}
		prev = &v
	}
	if at+1 < len(final) {
		if v, ok := FiniteOrder(final[at+1].Properties[prop]); ok {
			next = &v
		}
	}
	switch {
	case prev == nil && next == nil:
		return 1, true
	case prev == nil:
		return *next - 1, true
	case next == nil:
		return *prev + 1, true
	}
	// Twice the threshold, so the midpoint leaves at least the threshold on
	// both sides. A smaller gap reads as collapsed, and the next update's
	// renumber would then rewrite the whole family, including edges the
	// mover cannot see. The densify below touches only the siblings given.
	if *next-*prev < 2*OrderCollapseThreshold {
		return 0, false
	}
	return MidpointOrder(*prev, *next)
}
