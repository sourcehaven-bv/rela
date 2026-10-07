package store

import (
	"context"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// MentionIndex is the optional capability behind backlinks (TKT-Q6AKUN): it
// answers "which live entity states mention this id" from an index each
// backend keeps in step with its writes. A mention is what
// mentions.Targets extracts from a body; targets are indexed whether or not
// an entity with that id exists, so a mention written before its target
// becomes a backlink once the target is created.
//
// It returns raw refs and runs no ACL. Read surfaces must go through
// visibility.Resolver.MentionedBy, which gates and redacts every source;
// a guard test pins that no other package asserts this interface.
type MentionIndex interface {
	// MentionSources returns up to limit live entity states whose body
	// mentions target, ordered by [CompareRefs], strictly after the cursor
	// after. The zero Ref starts from the beginning. A soft-deleted state is
	// not live. limit must be positive; otherwise [ErrInvalidQuery].
	MentionSources(ctx context.Context, target string, after entity.Ref, limit int) ([]entity.Ref, error)
}

// CompareRefs orders refs by id, then by face token, both byte-wise. It is
// the order [MentionIndex.MentionSources] pages in, and matches SQL's
// `ORDER BY id, face` under the C collation the database backends use.
func CompareRefs(a, b entity.Ref) int {
	if c := strings.Compare(a.ID, b.ID); c != 0 {
		return c
	}
	return strings.Compare(a.Face.String(), b.Face.String())
}
