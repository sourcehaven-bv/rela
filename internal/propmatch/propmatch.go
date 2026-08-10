// Package propmatch is the single authoritative definition of what
// "empty" means for an entity property, plus the untyped string
// comparisons that can be decided without consulting the metamodel.
//
// It exists because two layers need the same emptiness rule but sit on
// opposite sides of a dependency boundary: [internal/filter] is
// metamodel-aware (typed date/int/enum comparison) and may not be
// imported by the store, while [internal/store] and its backends need
// to push a `status=doing` or `billing_email=` predicate down into the
// query. Duplicating the rule in the store layer would create a second
// notion of empty that drifts from the first — the failure this package
// exists to prevent.
//
// # Pure leaf
//
// This package imports nothing but stdlib, deliberately. It is depended
// on by both `filter` (a branch: metamodel, pattern, natsort) and
// `graphquerynaive` (allowed only `entity` + `store`), so any import
// added here lands in the store layer too. Keep it stdlib-only.
//
// # Scope: emptiness and string equality, NOT typed comparison
//
// [Decide] answers only the questions decidable without a property's
// declared type: is it empty, and does its string form equal a value.
// Ordered comparison (`due < 2026-01-01`) is deliberately absent — it
// needs the metamodel's declared layout to avoid comparing dates
// lexicographically, so it stays in [internal/filter] above the store.
// Callers get [Undecided] for those and fall back.
//
// # Callers do not all route every shape here
//
// [internal/filter] intercepts list-valued properties before delegating
// (it has richer per-operator list handling for regex/fuzzy), so only
// its empty-list case reaches [Decide]. The store backends, by
// contrast, route every shape through here. Both must agree — the
// storetest conformance suite's Props_value_shapes case pins scalars,
// lists, empty lists, ints and bools across backends.
package propmatch

import (
	"fmt"
	"slices"
)

// Op is a comparison this package can decide without type information.
type Op int

const (
	// OpEqual is `property=value`, and with an empty Value, `property=`
	// meaning "is empty".
	OpEqual Op = iota
	// OpNotEqual is `property!=value`, and with an empty Value,
	// `property!=` meaning "is not empty".
	OpNotEqual
)

// Result is the outcome of [Decide].
type Result int

const (
	// NoMatch means the predicate is decided and false.
	NoMatch Result = iota
	// Match means the predicate is decided and true.
	Match
	// Undecided means this package cannot answer without the property's
	// declared type — the caller must fall back to a metamodel-aware
	// comparison. Only ordered/pattern operators reach this; a caller
	// that supports only [OpEqual] / [OpNotEqual] never sees it.
	Undecided
)

// IsEmpty reports whether a raw property value counts as absent.
//
// A missing key and a present-but-empty value are the SAME state: YAML
// frontmatter parses a valueless key to nil, and an operator asking "is
// this field filled in?" does not distinguish the two. Both are empty.
//
// A non-empty list is not empty; an empty list is. This mirrors how a
// multi-select with nothing selected reads to a human.
func IsEmpty(val any) bool {
	switch v := val.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	default:
		return false
	}
}

// Decide evaluates op against a raw property value.
//
// The emptiness rule (matching internal/filter's documented semantics,
// which this package now backs):
//
//   - property=value  -> NoMatch  (empty is not equal to a value)
//   - property!=value -> NoMatch  (empty must not match "not equal to")
//   - property=       -> Match    (empty matches "is empty")
//   - property!=      -> NoMatch  (empty does not match "is not empty")
//
// Note the asymmetry on the second line: an empty property does NOT
// satisfy `status!=doing`. That is deliberate and long-standing — a
// filter names the population it wants, and an entity with no status is
// not in the "status is something other than doing" population. Treating
// it as a match would silently widen every exclusion filter to include
// unset rows.
//
// For a non-empty value, equality compares the value's string form.
// Lists match if ANY element equals the target (multi-select semantics).
func Decide(val any, op Op, target string) Result {
	if IsEmpty(val) {
		if op == OpEqual && target == "" {
			return Match
		}
		return NoMatch
	}

	// Value is non-empty. An empty target here is the existence check.
	if target == "" {
		if op == OpNotEqual {
			return Match // "is not empty", and it isn't
		}
		return NoMatch // "is empty", but it is not
	}

	eq := equalsTarget(val, target)
	if op == OpNotEqual {
		eq = !eq
	}
	if eq {
		return Match
	}
	return NoMatch
}

// equalsTarget reports whether val equals target by string form,
// treating a list as matching when any element does.
func equalsTarget(val any, target string) bool {
	switch v := val.(type) {
	case string:
		return v == target
	case []string:
		return slices.Contains(v, target)
	case []any:
		for _, item := range v {
			if Stringify(item) == target {
				return true
			}
		}
		return false
	default:
		return Stringify(val) == target
	}
}

// Stringify renders a scalar property value the way a filter comparison
// sees it. Kept here (rather than fmt.Sprint at each call site) so the
// store and filter layers agree on how a bool or number compares to a
// string literal.
func Stringify(val any) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprint(val)
}
