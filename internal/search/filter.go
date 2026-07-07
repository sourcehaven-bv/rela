package search

import (
	"errors"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ErrOrderedFilterUnsupported is returned when a [Query] uses an ordered
// property filter (FilterGt/Lt/Gte/Lte). The search backend matches on
// raw stringified attribute values and has no property-type context, so
// an ordered comparison here could only be lexicographic — "10" < "9" —
// which is silently wrong for integer/date properties. Callers that need
// typed ordering must use the metamodel-aware filter path
// (internal/filter.Match), not search property filters.
var ErrOrderedFilterUnsupported = errors.New(
	"search: ordered property filters (>, <, >=, <=) are unsupported; " +
		"use the metamodel-typed filter path for typed comparison")

// ValidateFilters rejects filters the search backend cannot evaluate
// correctly. Today that is the ordered operators, which would be
// lexicographic-only (see [ErrOrderedFilterUnsupported]). Callers
// validate once up front rather than discovering the problem per-entity.
func ValidateFilters(filters []PropertyFilter) error {
	for _, f := range filters {
		switch f.Op {
		case FilterGt, FilterLt, FilterGte, FilterLte:
			return ErrOrderedFilterUnsupported
		default:
		}
	}
	return nil
}

// MatchFilters returns true if an entity matches all property filters.
// Ordered operators are NOT handled here — they are rejected up front by
// [ValidateFilters]; if one reaches this function it is treated as a
// non-match (defensive: the Service validates before iterating).
func MatchFilters(e *entity.Entity, filters []PropertyFilter) bool {
	for _, f := range filters {
		val := e.GetAttributeString(f.Property)
		switch f.Op {
		case FilterEq:
			if val != f.Value {
				return false
			}
		case FilterNe:
			if val == f.Value {
				return false
			}
		case FilterContains:
			if !strings.Contains(strings.ToLower(val), strings.ToLower(f.Value)) {
				return false
			}
		case FilterGt, FilterLt, FilterGte, FilterLte:
			// Unsupported (see ValidateFilters / ErrOrderedFilterUnsupported).
			// Defensive non-match in case validation was bypassed.
			return false
		case FilterIn:
			found := false
			for _, v := range strings.Split(f.Value, ",") {
				if val == strings.TrimSpace(v) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case FilterExists:
			if e.GetAttribute(f.Property) == nil {
				return false
			}
		case FilterNotExists:
			if e.GetAttribute(f.Property) != nil {
				return false
			}
		}
	}
	return true
}

// Logical field names reported by match-provenance (see [MatchTextFields]).
// These form the vocabulary a [VisibleSearcher] intersects against a
// principal's visible-field set to close the match-on-hidden-field oracle.
//
//   - FieldID / FieldContent name the entity ID and body. Neither is ever
//     property-visibility-gated (only declared properties carry a `visible:`
//     grant), so a hit that matched on either always survives.
//   - PropFieldPrefix + the property name identifies a property match, e.g.
//     "prop:internal_notes". The suffix is the raw property key — the same
//     key space the affordance resolver's Visible map uses — so the consumer
//     can intersect the two sets directly.
const (
	FieldID         = "id"
	FieldContent    = "content"
	PropFieldPrefix = "prop:"
)

// MatchText returns true if any of the entity's ID, content, or string
// properties contain the search text (case-insensitive).
func MatchText(e *entity.Entity, text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(strings.ToLower(e.ID), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Content), lower) {
		return true
	}
	for _, v := range e.Properties {
		if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), lower) {
			return true
		}
	}
	return false
}

// MatchTextFields reports which logical fields of e the search text matches,
// using the same case-insensitive substring rule as [MatchText]. It is the
// ground-truth provenance implementation: the [FieldMatcher] every backend
// exposes must agree with it on the visible-field projection (pinned by
// storetest.RunVisibleSearchTests).
//
// Field names follow the [FieldID] / [FieldContent] / [PropFieldPrefix]
// vocabulary. The returned set is nil when nothing matches (mirroring
// MatchText returning false). Non-string properties never match (only string
// values are indexed), matching MatchText and both native backends.
func MatchTextFields(e *entity.Entity, text string) map[string]struct{} {
	lower := strings.ToLower(text)
	var fields map[string]struct{}
	add := func(name string) {
		if fields == nil {
			fields = make(map[string]struct{})
		}
		fields[name] = struct{}{}
	}
	if strings.Contains(strings.ToLower(e.ID), lower) {
		add(FieldID)
	}
	if strings.Contains(strings.ToLower(e.Content), lower) {
		add(FieldContent)
	}
	for name, v := range e.Properties {
		if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), lower) {
			add(PropFieldPrefix + name)
		}
	}
	return fields
}
