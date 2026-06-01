package metamodel

import "math"

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
func FiniteOrder(v interface{}) (float64, bool) {
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
