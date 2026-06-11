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
