package propmatch

import (
	"math"
	"strconv"
)

// Integer reports the integer a stored property value holds, under the one
// rule every reader of an `integer` property shares: the condition engine
// binds it as an Int (internal/predicatefns), and the store answers an
// integer range predicate with it (TKT-8HY05WB). Keeping one definition is
// what lets a store query replace the Go evaluation exactly.
//
// A value holds an integer when it is:
//
//   - a Go int or int64;
//   - a float64 with no fractional part in [-2^63, 2^63), the range an int64
//     can hold, so the conversion is exact;
//   - a string that strconv.ParseInt accepts in base 10: an optional sign,
//     then decimal digits, in int64 range.
//
// Anything else holds no integer, which the condition engine binds as nil.
func Integer(raw any) (int64, bool) {
	switch v := raw.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		// -2^63 is exactly representable; 2^63 is the first value out of
		// range. A NaN fails both comparisons.
		if v == math.Trunc(v) && v >= -(1<<63) && v < 1<<63 {
			return int64(v), true
		}
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}
