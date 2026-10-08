package metamodel

import (
	"cmp"
	"math"
	"strings"
)

// FiniteOrder converts a JSON- or YAML-decoded relation-property value to
// a finite float64. Returns (value, true) for any built-in Go numeric type
// (int*, uint*, float32, float64) that is finite; returns (0, false) for
// nil, non-numeric types, NaN, or +/-Inf.
//
// All consumers that interpret managed order properties go through this
// helper: the entity manager's auto-assign and renumber paths, the
// data-entry sort and wire validators, the analyzer, and the CLI
// commands. Keep one canonical implementation to avoid drift; do not
// redefine variants in callers.
func FiniteOrder(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case float32:
		f = float64(x)
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// OrderKey is one edge's place on a managed order side: its stored order
// value (as decoded, possibly missing) and the two fields that break ties,
// the edge's far endpoint and its tail face.
type OrderKey struct {
	Value any
	Peer  string
	Tail  string
}

// CompareOrderKeys is the one ordering of edges on a managed order side.
// Edges with a finite value come first, by value; edges without one follow.
// Ties, and edges without a value, fall back to the far endpoint's id and
// then the tail, so every reader and the move path agree on one order
// whatever order the store returned the rows in.
//
// The data-entry SPA mirrors it for optimistic reorder; keep the two in step.
func CompareOrderKeys(a, b OrderKey) int {
	av, aok := FiniteOrder(a.Value)
	bv, bok := FiniteOrder(b.Value)
	switch {
	case aok && !bok:
		return -1
	case !aok && bok:
		return 1
	case aok && bok && av != bv:
		return cmp.Compare(av, bv)
	}
	if c := strings.Compare(a.Peer, b.Peer); c != 0 {
		return c
	}
	return strings.Compare(a.Tail, b.Tail)
}
