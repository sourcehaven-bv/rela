// Package storeutil provides shared helpers for store.Store implementations.
//
// Functions here are used by both memstore and fsstore to avoid duplicating
// validation, filtering, and sorted-slice maintenance logic.
package storeutil

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ValidateID rejects IDs that would cause key collisions in the
// from--type--to relation key format.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("store: empty ID")
	}
	if strings.Contains(id, "--") {
		return fmt.Errorf("store: ID %q contains consecutive dashes", id)
	}
	return nil
}

// ValidateProperty rejects property names that would cause
// attachment key collisions in the entityID/property format.
func ValidateProperty(prop string) error {
	if prop == "" {
		return fmt.Errorf("store: empty property name")
	}
	if strings.Contains(prop, "/") {
		return fmt.Errorf("store: property name %q contains slash", prop)
	}
	return nil
}

// SortedInsert adds key to a sorted slice, maintaining sort order.
func SortedInsert(s []string, key string) []string {
	i, _ := slices.BinarySearch(s, key)
	return slices.Insert(s, i, key)
}

// SortedRemove removes key from a sorted slice.
// The key must exist — callers should only call this after confirming presence.
func SortedRemove(s []string, key string) []string {
	i, found := slices.BinarySearch(s, key)
	if !found {
		panic("storeutil: SortedRemove called with missing key: " + key)
	}
	return slices.Delete(s, i, i+1)
}

// MatchRelation returns true if a relation matches the given query.
func MatchRelation(r *entity.Relation, q store.RelationQuery) bool {
	if q.Type != "" && r.Type != q.Type {
		return false
	}
	if q.From != "" && r.From != q.From {
		return false
	}
	if q.To != "" && r.To != q.To {
		return false
	}
	if q.EntityID != "" {
		switch q.Direction {
		case store.DirectionOutgoing:
			if r.From != q.EntityID {
				return false
			}
		case store.DirectionIncoming:
			if r.To != q.EntityID {
				return false
			}
		default: // DirectionBoth
			if r.From != q.EntityID && r.To != q.EntityID {
				return false
			}
		}
	}
	return true
}

// MatchFilters returns true if an entity matches all property filters.
func MatchFilters(e *entity.Entity, filters []store.PropertyFilter) bool {
	for _, f := range filters {
		val := e.GetAttributeString(f.Property)
		switch f.Op {
		case store.FilterEq:
			if val != f.Value {
				return false
			}
		case store.FilterNe:
			if val == f.Value {
				return false
			}
		case store.FilterContains:
			if !strings.Contains(strings.ToLower(val), strings.ToLower(f.Value)) {
				return false
			}
		case store.FilterGt:
			if val <= f.Value {
				return false
			}
		case store.FilterLt:
			if val >= f.Value {
				return false
			}
		case store.FilterGte:
			if val < f.Value {
				return false
			}
		case store.FilterLte:
			if val > f.Value {
				return false
			}
		case store.FilterIn:
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
		case store.FilterExists:
			if e.GetAttribute(f.Property) == nil {
				return false
			}
		case store.FilterNotExists:
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
