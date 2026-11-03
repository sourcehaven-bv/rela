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
	// after. On a store with [RetiredIDs], a body that mentions an id retired
	// into target counts as mentioning target. The zero Ref starts from the beginning. A soft-deleted state is
	// not live. limit must be positive; otherwise [ErrInvalidQuery].
	MentionSources(ctx context.Context, target string, after entity.Ref, limit int) ([]entity.Ref, error)
}

// RetiredIDs is the optional capability of a store that remembers the ids a
// rename retired (TKT-J6GISC3). The database backends keep it; fsstore and
// memstore do not, and rewrite mention text in place instead (see
// [EntityWriter.RenameFamily]).
//
// A retired id stays reserved: [EntityWriter.CreateEntity] and
// [EntityWriter.RenameFamily] refuse it with [ErrConflict]. The one
// exception is renaming an entity back onto an id it retired itself, which
// drops that retirement. Retiring an id whose successor is renamed again
// moves it on to the newest id, so a lookup never walks a chain.
//
// It runs no ACL. Read surfaces resolve through visibility.Resolver, which
// gates the successor like any other id.
type RetiredIDs interface {
	// Successors maps each id among ids that a rename retired to the id the
	// entity carries now. An id that was never retired is absent. The
	// successor may itself no longer exist, when the entity was deleted
	// after its rename. Both ids stay reserved: no create or rename may take
	// a retired id or a successor id, except the entity renaming back onto
	// an id it retired itself. A case-only rename re-points the successors to
	// the new spelling and retires nothing.
	Successors(ctx context.Context, ids []string) (map[string]string, error)
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
