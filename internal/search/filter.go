package search

import (
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// MatchFilters returns true if an entity matches all property filters.
//
//nolint:gocognit // filter evaluation is a dense switch over operator cases; splitting hurts readability
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
		case FilterGt:
			if val <= f.Value {
				return false
			}
		case FilterLt:
			if val >= f.Value {
				return false
			}
		case FilterGte:
			if val < f.Value {
				return false
			}
		case FilterLte:
			if val > f.Value {
				return false
			}
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
