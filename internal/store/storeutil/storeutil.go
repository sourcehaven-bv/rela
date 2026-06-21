// Package storeutil provides shared helpers for store.Store implementations.
//
// Functions here are used by both memstore and fsstore to avoid duplicating
// validation, filtering, and sorted-slice maintenance logic.
package storeutil

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ValidateID rejects IDs that would cause key collisions or bucket-key
// corruption across store backends.
//
// In addition to the `--` separator rule (which collides with the
// from--type--to relation key format), IDs cannot contain path
// separators, NUL, or ASCII control characters. Those would break
// nested-bucket range scans in backends that key on bucket hierarchy
// (see internal/store/boltstore) and have always been latent hazards
// in fsstore (NUL crashes file creation on POSIX; `/` silently creates
// nested directories).
func ValidateID(id string) error {
	if id == "" {
		return errors.New("store: empty ID")
	}
	if strings.Contains(id, "--") {
		return fmt.Errorf("store: ID %q contains consecutive dashes", id)
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("store: ID %q contains path separator", id)
	}
	for i := range len(id) {
		if id[i] < 0x20 || id[i] == 0x7f {
			return fmt.Errorf("store: ID %q contains control character", id)
		}
	}
	return nil
}

// ValidateRelationType rejects relation types that would cause
// relation-key collisions or storage hazards. The rules mirror
// [ValidateID]: relation types are embedded in the same
// from--type--to key format (so `--` collides), become path segments
// in fsstore relation filenames (so separators nest directories), and
// appear in the pgstore change-feed payload whose field separator is
// a control character (internal/store/pgstore/feed.go already
// documents that assumption). Previously each backend hand-rolled a
// subset of these checks inline; the shared rule is also the validity
// oracle for the storetest fuzz harness (TKT-PCLGGL).
func ValidateRelationType(relType string) error {
	if relType == "" {
		return errors.New("store: empty relation type")
	}
	if strings.Contains(relType, "--") {
		return fmt.Errorf("store: relation type %q contains consecutive dashes", relType)
	}
	if strings.ContainsAny(relType, "/\\") {
		return fmt.Errorf("store: relation type %q contains path separator", relType)
	}
	for i := range len(relType) {
		if relType[i] < 0x20 || relType[i] == 0x7f {
			return fmt.Errorf("store: relation type %q contains control character", relType)
		}
	}
	return nil
}

// ValidateProperty rejects property names that would cause
// attachment key collisions in the entityID/property format.
func ValidateProperty(prop string) error {
	if prop == "" {
		return errors.New("store: empty property name")
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

// LimitAttachmentReader wraps r so reads fail with
// store.ErrAttachmentTooLarge once they exceed store.MaxAttachmentBytes.
// This is the shared backstop guard every store backend applies to
// AttachFile, so no backend is ever unbounded regardless of caller.
// Thin alias over store.CapAttachmentReader at the backstop cap.
func LimitAttachmentReader(r io.Reader) io.Reader {
	return store.CapAttachmentReader(r, store.MaxAttachmentBytes)
}
