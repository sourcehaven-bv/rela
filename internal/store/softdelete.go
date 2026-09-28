package store

import (
	"context"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// SoftDeleter is the optional capability behind the data-entry Undo toast. It
// takes a whole entity family and its incident relations out of the live store
// without destroying them, so a delete can be undone for a short while before a
// garbage-collection job removes the rows for real.
//
// A marked entity is not a recycle-bin entry. Every read on the store treats it
// as deleted: gets return [ErrNotFound], lists, counts, graph queries and search
// leave it out, and its incident relations disappear with it. What a mark keeps
// is:
//
//   - the id: creating or renaming onto a marked id (case-folded) returns
//     [ErrConflict] until the mark is purged;
//   - the rows, unchanged, so [SoftDeleter.Unmark] brings back the same
//     content, relations and attachments;
//   - the highest sequence number, so [EntityReader.HighestID] still counts it.
//
// Marking emits the same events and observer calls as a delete, and unmarking
// emits the same ones as a create, so derived state (search indexes, the SSE
// feed, other processes) needs no knowledge of marks.
//
// Methods called on a transaction view join that transaction.
type SoftDeleter interface {
	// MarkDeleted marks every face of id and hides every relation touching
	// it. by names the principal who deleted it. Returns ErrNotFound when id
	// has no live face. The result lists what was hidden.
	MarkDeleted(ctx context.Context, id, by string) (*DeleteResult, error)

	// Unmark brings a marked family back, with the relations that were hidden
	// with it. A relation whose other endpoint is itself still marked stays
	// hidden and follows that entity instead. Returns ErrNotFound when id is
	// not marked. The result lists what came back.
	Unmark(ctx context.Context, id string) (*DeleteResult, error)

	// ListMarked returns every marked family, oldest mark first.
	ListMarked(ctx context.Context) ([]MarkedEntity, error)

	// PurgeMarked removes a marked family for real: its rows, every hidden
	// relation touching it and its attachments. Returns ErrNotFound when id
	// is not marked. The result lists what was removed.
	PurgeMarked(ctx context.Context, id string) (*DeleteResult, error)
}

// SoftDeleteProvider is a store that supports [SoftDeleter]. Type-asserted at
// the call site, like [VersionServiceProvider].
//
// Nil: never returned by the in-tree backends.
type SoftDeleteProvider interface {
	SoftDelete() SoftDeleter
}

// MarkedEntity is one marked family.
type MarkedEntity struct {
	ID        string
	DeletedAt time.Time
	// DeletedBy is the principal user recorded at mark time; empty when the
	// delete carried none.
	DeletedBy string
	// Entities holds every face, default face first.
	Entities []*entity.Entity
}

// revealKey is the context key for [WithRevealed].
type revealKey struct{}

// WithRevealed returns a context under which [RelationReader.GetRelation] and
// [RelationReader.ListRelations] also return the hidden relations of ONE marked
// entity. Nothing else honors it: entity reads, counts, pages and graph queries
// still treat the entity and its relations as deleted.
//
// It exists for one caller. Restoring an entity must be authorized against the
// same local roles that authorized deleting it, and the ACL resolves those roles
// from the entity's own relations (acl.StoreGraph reads exactly these two
// methods). Without the reveal those relations are hidden, so a restore would be
// judged without the grants the delete was judged with.
//
// For GetRelation the id must be one endpoint. For ListRelations the query's
// EntityID must equal it; the hidden relations then follow the live ones, so the
// usual ordering does not hold across the two groups.
func WithRevealed(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, revealKey{}, id)
}

// RevealedID returns the id set by [WithRevealed], or "" when none is.
func RevealedID(ctx context.Context) string {
	id, _ := ctx.Value(revealKey{}).(string)
	return id
}

// RevealedFor reports whether ctx reveals the hidden relations of an entity
// that is one endpoint of the relation from→to.
func RevealedFor(ctx context.Context, from, to string) (string, bool) {
	id := RevealedID(ctx)
	if id == "" || (id != from && id != to) {
		return "", false
	}
	return id, true
}
