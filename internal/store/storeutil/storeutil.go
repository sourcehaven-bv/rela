// Package storeutil provides shared helpers for store.Store implementations.
//
// Functions here are used by both memstore and fsstore to avoid duplicating
// validation, filtering, and sorted-slice maintenance logic.
package storeutil

import (
	"encoding/base64"
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

// EncodeCursor turns a sort key into an opaque pagination cursor.
// Callers MUST NOT parse cursors — round-trip only via DecodeCursor.
func EncodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

// DecodeCursor recovers the sort key from a cursor produced by EncodeCursor.
// Returns an error for malformed input; an empty cursor decodes to "".
func DecodeCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("store: invalid cursor: %w", err)
	}
	return string(b), nil
}

// PageKeys holds the result of a paginated key scan: the sort keys that
// landed on this page (in order) and a cursor pointing past the last
// emitted key when more results exist.
type PageKeys struct {
	Keys       []string
	NextCursor string
}

// PaginateSortedKeys walks a pre-sorted slice of keys, selecting the
// next page of keys that satisfy match. It starts strictly after
// cursorKey — the key returned as the previous NextCursor is not
// re-emitted. When limit <= 0, every matching key is returned and
// NextCursor is "".
//
// NextCursor is set iff a matching key exists after the last emitted
// key, so an empty NextCursor is a reliable "no more results" signal.
// This costs one extra match call past the cut-off on the final page.
//
// Callers load the concrete items from the returned keys — keeping
// loads out of this helper lets each backend handle load errors in
// its own idiom (skip missing for in-memory, propagate I/O errors
// for fsstore).
func PaginateSortedKeys(
	sortedKeys []string,
	cursorKey string,
	limit int,
	match func(key string) bool,
) PageKeys {
	start := 0
	if cursorKey != "" {
		i, found := slices.BinarySearch(sortedKeys, cursorKey)
		start = i
		if found {
			start = i + 1
		}
	}

	page := PageKeys{}
	for i := start; i < len(sortedKeys); i++ {
		key := sortedKeys[i]
		if !match(key) {
			continue
		}
		if limit > 0 && len(page.Keys) == limit {
			// Already full and found another match — more results exist.
			page.NextCursor = EncodeCursor(page.Keys[len(page.Keys)-1])
			return page
		}
		page.Keys = append(page.Keys, key)
	}
	return page
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
