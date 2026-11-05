package metamodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// PropertyTypeDecimal is an exact decimal number, such as a price or a
// number of hours.
//
// # Stored form
//
// A decimal is stored as a string in plain decimal notation: an optional
// sign, digits, and optionally a point followed by digits ("12.50", "-3",
// "0.125"). Exponents, a leading or trailing point, and separators are not
// decimal notation. The string is kept exactly as written, so "12.50" stays
// "12.50" through every store and every write: rela does not canonicalise
// it. YAML writes such a string quoted (`price: "12.50"`).
//
// An unquoted YAML number (`price: 12.5`) and a JSON number are accepted
// too. They reach rela as a binary float, so they are read through the
// shortest decimal text that names the same float ("12.5"), and a trailing
// zero the author typed is already gone. Quote a decimal to keep its exact
// text.
//
// # Comparison
//
// Every comparison is numeric and exact: "12.50" equals "12.5", and "9"
// orders before "10". Values are parsed to [big.Rat], never to float64.
const PropertyTypeDecimal = "decimal"

// MaxDecimalScale is the largest `scale:` a decimal property may declare.
const MaxDecimalScale = 30

// maxDecimalDigits caps the length of a decimal's text. It keeps a hostile
// value from making every comparison slow; no price or quantity comes near
// it.
const maxDecimalDigits = 64

// ErrInvalidDecimal is wrapped by every [ParseDecimal] failure.
var ErrInvalidDecimal = errors.New("not a decimal number")

// ParseDecimal reads a decimal property value. It accepts a string in
// decimal notation (see [PropertyTypeDecimal]), a Go integer, a float64,
// and a [json.Number]. It returns the exact value and the number of digits
// after the point in the value's text, which the `scale:` check reads.
//
// Nil: rejected — a nil value is an empty property, which callers handle
// before parsing.
func ParseDecimal(v any) (*big.Rat, int, error) {
	text, err := DecimalText(v)
	if err != nil {
		return nil, 0, err
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, 0, fmt.Errorf("%w: %q", ErrInvalidDecimal, text)
	}
	scale := 0
	if i := strings.IndexByte(text, '.'); i >= 0 {
		scale = len(text) - i - 1
	}
	return r, scale, nil
}

// DecimalText returns the decimal notation of a decimal property value, or
// an error when the value is not one. A string is returned as written once
// it passes the notation check; a number is rendered as its shortest exact
// decimal text.
func DecimalText(v any) (string, error) {
	switch x := v.(type) {
	case string:
		if !isDecimalNotation(x) {
			return "", fmt.Errorf("%w: %q", ErrInvalidDecimal, x)
		}
		return x, nil
	case json.Number:
		return DecimalText(string(x))
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case int32:
		return strconv.FormatInt(int64(x), 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return "", fmt.Errorf("%w: %v", ErrInvalidDecimal, x)
		}
		s := strconv.FormatFloat(x, 'f', -1, 64)
		if len(s) > maxDecimalDigits {
			return "", fmt.Errorf("%w: %v has more than %d digits", ErrInvalidDecimal, x, maxDecimalDigits)
		}
		return s, nil
	}
	return "", fmt.Errorf("%w: a %T", ErrInvalidDecimal, v)
}

// isDecimalNotation reports whether s is `[+-]?digits(.digits)?` and no
// longer than maxDecimalDigits.
func isDecimalNotation(s string) bool {
	if s == "" || len(s) > maxDecimalDigits {
		return false
	}
	if s[0] == '+' || s[0] == '-' {
		s = s[1:]
	}
	intPart, frac, hasPoint := strings.Cut(s, ".")
	if !allDigits(intPart) {
		return false
	}
	return !hasPoint || allDigits(frac)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// CompareDecimals compares two decimal property values numerically. ok is
// false when either value is not a decimal.
func CompareDecimals(a, b any) (cmp int, ok bool) {
	ra, _, errA := ParseDecimal(a)
	rb, _, errB := ParseDecimal(b)
	if errA != nil || errB != nil {
		return 0, false
	}
	return ra.Cmp(rb), true
}

// InDecimalRange reports whether r lies within the property's declared Min
// and Max, inclusive. An unset bound does not constrain.
//
// Nil: rejected for r — callers parse the value first.
func (p PropertyDef) InDecimalRange(r *big.Rat) bool {
	if p.Min != nil && r.Cmp(new(big.Rat).SetInt64(int64(*p.Min))) < 0 {
		return false
	}
	return p.Max == nil || r.Cmp(new(big.Rat).SetInt64(int64(*p.Max))) <= 0
}

// IsNumericType reports whether a property type holds a number: integer or
// decimal. Code that treats the two alike branches on this rather than
// listing them, so a site cannot forget one.
func IsNumericType(t string) bool {
	return t == PropertyTypeInteger || t == PropertyTypeDecimal
}

func validateDecimalValue(propName string, propDef *PropertyDef, val any) *ValidationError {
	switch list := val.(type) {
	case []any:
		for _, item := range list {
			if err := validateDecimalScalar(propName, propDef, item); err != nil {
				return err
			}
		}
		return nil
	case []string:
		for _, item := range list {
			if err := validateDecimalScalar(propName, propDef, item); err != nil {
				return err
			}
		}
		return nil
	}
	return validateDecimalScalar(propName, propDef, val)
}

func validateDecimalScalar(propName string, propDef *PropertyDef, val any) *ValidationError {
	r, scale, err := ParseDecimal(val)
	if err != nil {
		if _, isStr := val.(string); !isStr && !isNumberKind(val) {
			return &ValidationError{Type: ValidationErrorInvalidType, Property: propName, Message: "Must be a decimal number"}
		}
		return &ValidationError{
			Type:     ValidationErrorInvalidValue,
			Property: propName,
			Message:  "Invalid decimal (use digits with an optional point, such as 12.50)",
		}
	}
	if propDef.Scale != nil && scale > *propDef.Scale {
		return &ValidationError{
			Type:     ValidationErrorInvalidValue,
			Property: propName,
			Message:  fmt.Sprintf("Too many digits after the point (at most %d allowed)", *propDef.Scale),
		}
	}
	if propDef.InDecimalRange(r) {
		return nil
	}
	return outOfRange(propName, propDef)
}

func isNumberKind(v any) bool {
	switch v.(type) {
	case int, int32, int64, uint64, float64, json.Number:
		return true
	}
	return false
}
