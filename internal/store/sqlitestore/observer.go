package sqlitestore

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Observers build derived state — search indexes, caches, projections — from
// store writes. They are the mechanism by which a search index stays current,
// so a backend without them silently serves stale search results.
//
// Distinct from the Event stream ([Store.Subscribe]): events are a coarse
// staleness signal that may be DROPPED when a subscriber is slow, whereas
// observer callbacks are synchronous and carry the entity itself. An index must
// use observers; a UI refresh nudge can use events.

// WithObserver registers an observer at construction. Observers cannot be added
// after Open: registration is not synchronized, and a store that gained an
// observer mid-flight would have an index missing every write that preceded it
// — a corruption that looks like a search bug much later.
func WithObserver(o store.EntityObserver) Option {
	return func(s *Store) {
		if o != nil {
			s.observers = append(s.observers, o)
		}
	}
}

// Option configures a Store at construction.
type Option func(*Store)

// notifyPut reports a create or update.
//
// Errors are deliberately ignored, matching fsstore and memstore: an observer
// is derived state, and failing a committed write because an index rejected it
// would leave the store and the index disagreeing in the harder direction —
// the write is already durable. An observer that can fail meaningfully must
// surface that itself.
func (s *Store) notifyPut(e *entity.Entity) {
	if s.txPending != nil {
		// Deferred for the same reason events are: an observer that fires
		// inside a transaction that later rolls back leaves the search index
		// holding an entity the store does not have. RunTxRollbackTests did
		// not catch this — it asserts on events and rows, never on observers.
		s.txPending.add(func(root *Store) { root.notifyPut(e) })
		return
	}
	for _, o := range s.root().observers {
		_ = o.EntityPut(e)
	}
}

func (s *Store) notifyDelete(id string) {
	if s.txPending != nil {
		s.txPending.add(func(root *Store) { root.notifyDelete(id) })
		return
	}
	for _, o := range s.root().observers {
		_ = o.EntityDelete(id)
	}
}

// notifyRenamed fans out a rename as EXACTLY ONE callback — never
// EntityDelete(oldID) + EntityPut(renamed). An index-style observer deletes the
// old key and indexes the new content atomically in its EntityRenamed body;
// splitting it would leave a window where the entity is in no index at all.
func (s *Store) notifyRenamed(oldID string, renamed *entity.Entity) {
	if s.txPending != nil {
		s.txPending.add(func(root *Store) { root.notifyRenamed(oldID, renamed) })
		return
	}
	for _, o := range s.root().observers {
		_ = o.EntityRenamed(oldID, renamed)
	}
}
