package storetest

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The recorders in this package ([Counting], [Breadth], [BodyWatch]) embed
// store.Store, and an embedded interface promotes only its own methods. A
// capability the wrapped store offers beyond that interface would therefore
// disappear behind a recorder, and a consumer that type-asserts it would take
// its fallback path: a delete would go hard instead of soft, a backlink read
// would find no index. The recorders forward these capabilities explicitly so
// that stacking them leaves the routing under test unchanged.
//
// store.TypeWatermark is the exception. A consumer picks its fallback by
// type-asserting it, so a method on a recorder would claim the capability
// for every wrapped store, including memstore, which has none. Without the
// method a recorder answers the assertion the way memstore does: the CalDAV
// collection tag takes its per-entry fallback, as it would over memstore
// unwrapped.

// errNoSoftDelete is returned by every method of the soft deleter a recorder
// hands out when the wrapped store has none.
var errNoSoftDelete = errors.New("storetest: the wrapped store does not support soft delete")

// forwardedSoftDeleter returns s's soft deleter. A store without one yields a soft
// deleter whose every call fails: a recorder always advertises the
// capability, so a missing one surfaces as an error rather than as a silent
// switch to the hard-delete path. Every in-tree backend has one.
func forwardedSoftDeleter(s store.Store) store.SoftDeleter {
	if p, ok := s.(store.SoftDeleteProvider); ok {
		return p.SoftDelete()
	}
	return noSoftDelete{}
}

// noSoftDelete is the soft deleter of a wrapped store that has none.
type noSoftDelete struct{}

func (noSoftDelete) MarkDeleted(context.Context, string, string) (*store.DeleteResult, error) {
	return nil, errNoSoftDelete
}

func (noSoftDelete) Unmark(context.Context, string) (*store.DeleteResult, error) {
	return nil, errNoSoftDelete
}

func (noSoftDelete) ListMarked(context.Context) ([]store.MarkedEntity, error) {
	return nil, errNoSoftDelete
}

func (noSoftDelete) PurgeMarked(context.Context, string) (*store.DeleteResult, error) {
	return nil, errNoSoftDelete
}

// mentionSourcesOf forwards to s's mention index. A store without one fails
// the call: no index means no answer, not an empty one.
func mentionSourcesOf(
	ctx context.Context, s store.Store, target string, after entity.Ref, limit int,
) ([]entity.Ref, error) {
	idx, ok := s.(store.MentionIndex)
	if !ok {
		return nil, errors.New("storetest: the wrapped store keeps no mention index")
	}
	return idx.MentionSources(ctx, target, after, limit)
}
