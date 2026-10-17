package channels

import (
	"context"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Store persists channels, their messages and their audit trail.
//
// It is declared here, beside the types it speaks, because its consumers
// are the channel service and the alias hook built on this package, and
// every backend must satisfy exactly this contract: channelstest.RunAll is
// the one definition of it.
//
// The store authorizes nothing and applies no lifecycle policy. It enforces
// the caps, key grammar, gapless seq, the compare-and-set steps and thread
// depth, because those must hold no matter which caller writes.
//
// Times are stored at microsecond precision in UTC (see [NormTime]).
//
// Nil: no method returns a nil error with a zero result for a missing
// channel or message; that is [ErrNotFound]. List methods return an empty,
// non-nil slice when nothing matches.
type Store interface {
	// Append adds a message and returns it with its seq and rev. A
	// cardinality-one channel (empty name) is created on its first append,
	// recording msg.Incarnation; a named channel must exist, or the result
	// is [ErrNotFound]. An existing channel recorded with another
	// incarnation is [ErrStaleAnchor]. A reply names an existing, unremoved
	// top-level message, whose reply count rises in the same step.
	Append(ctx context.Context, key Key, msg NewMessage) (Message, error)

	// Edit replaces a message's properties and sets its edit time. The old
	// properties are not kept. A removed message is [ErrRemoved].
	Edit(ctx context.Context, key Key, seq int64, props map[string]any, at time.Time) (Message, error)

	// Remove erases a message's content and records the removal. Only the
	// seq, the thread link and the removal survive, in storage as well as on
	// read. Writes an audit record without the reason.
	Remove(ctx context.Context, key Key, seq int64, removal Removal) (Message, error)

	// SetAction moves the message's action named name from status expect to
	// next, as one compare-and-set; a lost race is [ErrConflict]. A next
	// status of [ActionNone] clears the action, which is how a failed side
	// effect is reverted. Reaching [ActionDone] writes an audit record.
	SetAction(ctx context.Context, key Key, seq int64, name string, expect ActionStatus, next Action) (Message, error)

	// CreateMany creates a named channel on anchor, with a minted name, and
	// returns it. The anchor may hold at most [NewChannel.Cap] named
	// channels of the type, counting channels of every incarnation; the
	// check and the create are one step, so concurrent creates never
	// exceed it. Writes an audit record.
	CreateMany(ctx context.Context, anchor Anchor, nc NewChannel) (Channel, error)

	// SetLifecycle moves the channel from state expect to t.To as one
	// compare-and-set; a lost race is [ErrConflict]. Writes an audit record.
	SetLifecycle(ctx context.Context, key Key, expect State, t Transition) (Channel, error)

	// Delete removes the channel and every message in it, and writes a
	// deletion record to the entity's audit trail, which survives.
	Delete(ctx context.Context, key Key, by Author, at time.Time) error

	// Rekey moves every channel and the audit trail of entity oldID to
	// newID: afterwards nothing is found under oldID. An oldID without
	// channels moves only its audit trail. A newID that already has
	// channels is [ErrRekeyConflict] and nothing moves.
	//
	// The channels always move as one step, so they are never split
	// between the two ids. The database backends move the audit trail in
	// the same transaction. The file backend moves it in a second step
	// after the channels: a crash between the two leaves the trail under
	// oldID, where Audit(oldID) still reads it, and the next Rekey or
	// Tombstone of oldID moves or files it.
	Rekey(ctx context.Context, oldID, newID string, at time.Time) error

	// Tombstone moves channels out of reach without deleting them, and
	// returns the handle of the batch it moved. An empty face takes every
	// channel of the entity id, on every face, and files the audit trail
	// away with them, so a new entity reusing the id starts clean. A face
	// takes only the channels anchored to that face. An id without channels
	// is not an error: nothing moves and the handle is zero.
	//
	// Nothing in this interface reads a tombstoned channel again, except
	// [Store.Untombstone] given its handle: no list, sweep, Remove or Delete
	// reaches it, so neither retention nor erasure does. Purging tombstoned
	// content is the retention work of TKT-964SIS, which adds the method
	// that reaches it.
	Tombstone(ctx context.Context, id string, face entity.Face, at time.Time) (TombstoneHandle, error)

	// Untombstone puts back exactly the batch h names, with the audit trail
	// a whole-entity tombstone filed away, and records the restore in the
	// trail. It is how a caller undoes a tombstone it took before a write
	// that then failed.
	//
	// When live channels sit on the batch's anchor (any channel of the id
	// for a whole-entity batch, any channel of the face for a face batch),
	// it refuses with [*OccupiedError] and nothing changes. A batch already
	// restored, or never filed, is [ErrNotFound]. The zero handle is a
	// no-op.
	Untombstone(ctx context.Context, h TombstoneHandle, at time.Time) error

	// Head returns the channel's metadata, including the last seq and rev a
	// poller compares. It is cheap on every backend, and it reflects only
	// completed writes.
	Head(ctx context.Context, key Key) (Channel, error)

	// Get returns one message.
	Get(ctx context.Context, key Key, seq int64) (Message, error)

	// Messages returns one page of messages; see [Page] for the cursors.
	Messages(ctx context.Context, key Key, page Page) ([]Message, error)

	// List returns the channels at exactly anchor, ordered by type and
	// name. An empty typ matches every type; an empty state every state.
	List(ctx context.Context, anchor Anchor, typ string, state State) ([]Channel, error)

	// ListEntity returns every channel of entity id, on every face, ordered
	// by face, type and name.
	ListEntity(ctx context.Context, id string) ([]Channel, error)

	// ListByLifecycle returns the channels in state that entered it before
	// the given time (see [Channel.StateSince]), for the retention and
	// archive sweeps.
	ListByLifecycle(ctx context.Context, state State, before time.Time) ([]Channel, error)

	// StalePending returns the actions left pending since before the given
	// time, for the sweep that settles them.
	StalePending(ctx context.Context, before time.Time) ([]PendingAction, error)

	// Audit returns the live audit trail of entity id, oldest first.
	Audit(ctx context.Context, id string) ([]AuditRecord, error)

	// Keys returns the key of every live channel, for export and import
	// between backends.
	Keys(ctx context.Context) ([]Key, error)

	// TypeRecords returns the record of each channel type the store holds
	// one for (see [TypeRecord]). A store that recorded nothing returns an
	// empty map. Compare it with the configuration through
	// [TypeRecordChanges].
	TypeRecords(ctx context.Context) (map[string]TypeRecord, error)

	// RecordTypes records each given type that the store holds no record
	// for, and adds the given refs to the record of a type it does hold.
	// It never replaces an anchor mode or cardinality: a record that
	// differs from the held one in either, or whose Plain names a held
	// ref, is [ErrTypeShapeConflict] (see [MergeTypeRecord]). It writes
	// nothing when nothing is new. Types not given keep theirs. An invalid
	// type is [ErrInvalidKey], an unknown mode or cardinality
	// [ErrInvalidMessage]; on any error nothing is recorded.
	RecordTypes(ctx context.Context, records map[string]TypeRecord) error
}
