package rollup

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/propmatch"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// propHolds decides one predicate on a single stored value raw (nil for an
// unset property), as the reference matcher does, for the operators a node
// condition lowers to. Any other operator or order kind is an error, so the
// caller keeps the condition on the Go path rather than guessing.
func propHolds(p store.PropPredicate, raw any) (bool, error) {
	switch p.Op {
	case store.PropEqual:
		if p.Scalar && p.Value != "" {
			s, ok := raw.(string)
			return ok && s == p.Value, nil
		}
		return propmatch.Decide(raw, propmatch.OpEqual, p.Value) == propmatch.Match, nil
	case store.PropNotEqual:
		return propmatch.Decide(raw, propmatch.OpNotEqual, p.Value) == propmatch.Match, nil
	case store.PropNotEqualOrEmpty:
		if p.Scalar && p.Value != "" {
			s, ok := raw.(string)
			return !ok || s != p.Value, nil
		}
		return propmatch.IsEmpty(raw) || propmatch.Decide(raw, propmatch.OpNotEqual, p.Value) == propmatch.Match, nil
	case store.PropEqualOrEmpty:
		if propmatch.IsEmpty(raw) {
			return true, nil
		}
		equal := p
		equal.Op = store.PropEqual
		return propHolds(equal, raw)
	case store.PropGreaterEqual, store.PropLessEqual, store.PropGreater, store.PropLess:
		return orderedHolds(p, raw)
	default:
		return false, fmt.Errorf("rollup: no default fill for operator %v on %q", p.Op, p.Property)
	}
}

// orderedHolds decides an ordered predicate of an integer or string kind.
func orderedHolds(p store.PropPredicate, raw any) (bool, error) {
	var c int
	switch p.Kind {
	case store.OrderInteger:
		want, err := strconv.ParseInt(p.Value, 10, 64)
		if err != nil {
			return false, fmt.Errorf("rollup: integer bound %q on %q: %w", p.Value, p.Property, err)
		}
		n, ok := propmatch.Integer(raw)
		if !ok {
			return false, nil
		}
		c = cmp.Compare(n, want)
	case store.OrderString:
		s, ok := raw.(string)
		if !ok {
			return false, nil
		}
		c = strings.Compare(s, p.Value)
	default:
		return false, fmt.Errorf("rollup: no default fill for order kind %v on %q", p.Kind, p.Property)
	}
	switch p.Op {
	case store.PropLess:
		return c < 0, nil
	case store.PropLessEqual:
		return c <= 0, nil
	case store.PropGreater:
		return c > 0, nil
	default:
		return c >= 0, nil
	}
}
