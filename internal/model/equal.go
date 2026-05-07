package model

import "reflect"

// PropertyValueEqual returns true if two property values represent the
// same logical value. Numeric types (int, int32, int64, float32,
// float64) are compared by float64 value because JSON unmarshal always
// produces float64 while disk-loaded YAML can produce either an int or
// a float depending on the literal — so a JSON-decoded `weight: 5`
// must compare equal to a disk-loaded `weight: 5`.
//
// Type boundaries other than the numeric one are respected: int(5) is
// NOT equal to "5", and bool(true) is NOT equal to "true". For nested
// maps and slices we delegate to reflect.DeepEqual.
func PropertyValueEqual(a, b interface{}) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	if af, aok := toFloat(a); aok {
		if bf, bok := toFloat(b); bok {
			return af == bf
		}
	}
	return false
}

// PropertyMapsEqual returns true if two property maps have identical
// keys and PropertyValueEqual values. Nil and empty maps are treated as
// equal.
func PropertyMapsEqual(a, b map[string]interface{}) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if !PropertyValueEqual(va, vb) {
			return false
		}
	}
	return true
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
