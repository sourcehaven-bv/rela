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
	//
	// With msg.Session set, the append may move the channel's session in
	// the same step: the store calls the step's decider with the channel
	// as committed and, on a move, stores a notice right after the message,
	// sets the session and writes the session audit record (see
	// [SessionStep]). A move with an invalid state name is
	// [ErrInvalidMessage] and stores nothing. A step does not imply
	// RequireOpen: the decider sees the lifecycle state and decides. Any
	// other append whose TriggeredBy starts with [SessionMarker] is
	// [ErrInvalidMessage]. The returned message is the causing one. The
	// step's Due is required, and its result is stored after the move or
	// non-move; a step that names a Timer is [ErrInvalidMessage].
	Append(ctx context.Context, key Key, msg NewMessage) (Message, error)

	// SetSession runs step against the channel's session without a
	// message: a timer fire when the decider returns a move, a reschedule
	// otherwise. On a move it appends a notice at last_seq+1 (see
	// [TimerNotice]), applies the move at at, and writes the session audit
	// record (see [TimerAudit]). A move without step.Timer, or with an
	// invalid state name, is [ErrInvalidMessage]. In every case it then
	// applies step.Due. A move raises rev; a reschedule does not. Readers
	// do see a due time, but the wire derives it from the configuration
	// and the session's state, entry time and restart anchors, never from
	// the stored Session.NextDueAt, which is only an index hint for
	// [Store.ListSessionsDue]. A reschedule changes only that hint, so
	// nothing a reader sees changes. A missing channel is [ErrNotFound]; it
	// never creates one.
	SetSession(ctx context.Context, key Key, at time.Time, step SessionStep) (Channel, error)

	// ListSessionsDue returns the live, open channels of the given types
	// whose Session.NextDueAt is set and at or before until, ordered by
	// NextDueAt and then by key (anchor id, face, type, name), at most
	// limit. types must hold at least one type, all valid, else
	// [ErrInvalidKey]; a limit of 0 or less is [ErrInvalidMessage].
	//
	// The database backends read an index. The file backend walks the tree
	// once per call: one directory read of the root, of each entity
	// directory and of each face directory, and one meta.yaml read per
	// channel of the given types. It skips a damaged meta.yaml with a
	// warning.
	ListSessionsDue(ctx context.Context, types []string, until time.Time, limit int) ([]Channel, error)

	// Edit replaces a message's properties and sets its edit time. The old
	// properties are not kept. A removed message is [ErrRemoved], and a
	// message with a pending action is [ErrActionPending].
	Edit(ctx context.Context, key Key, seq int64, props map[string]any, at time.Time) (Message, error)

	// Remove erases a message's content and records the removal. Only the
	// seq, the thread link and the removal survive, in storage as well as on
	// read. Writes an audit record without the reason. A message with a
	// pending action is [ErrActionPending].
	Remove(ctx context.Context, key Key, seq int64, removal Removal) (Message, error)

	// SetAction moves the message's action named name from status expect to
	// next, as one compare-and-set; a lost race is [ErrConflict]. A next
	// status of [ActionNone] clears the action, which is how a failed side
	// effect is reverted. Reaching [ActionDone] writes an audit record.
	// Clearing a pending action is a release: next must carry By and At
	// (see [CheckRelease]), and an audit record with status
	// [AuditReleased] is written. Taking an action (a step from
	// [ActionNone]) in a channel that is not open is [ErrNotOpen] (see
	// [CheckActionState]).
	SetAction(ctx context.Context, key Key, seq int64, name string, expect ActionStatus, next Action) (Message, error)

	// CreateMany creates a named channel on anchor, with a minted name, and
	// returns it. The anchor may hold at most [NewChannel.Cap] named
	// channels of the type, counting channels of every incarnation; the
	// check and the create are one step, so concurrent creates never
	// exceed it. Writes an audit record. With nc.SessionDue set, its
	// result is stored in the same step.
	CreateMany(ctx context.Context, anchor Anchor, nc NewChannel) (Channel, error)

	// SetLifecycle moves the channel from state expect to t.To as one
	// compare-and-set; a lost race is [ErrConflict]. Writes an audit record.
	// With t.SessionDue set, its result is stored in the same step; without
	// it the session schedule is kept as it was.
	SetLifecycle(ctx context.Context, key Key, expect State, t Transition) (Channel, error)

	// Delete removes the channel and every message in it, and writes a
	// deletion record to the entity's audit trail, which survives. It is a
	// compare-and-set on the state and rev the caller read (see
	// [Deletion]): a channel that moved or changed since is [ErrConflict]
	// and nothing is deleted.
	Delete(ctx context.Context, key Key, d Deletion) error

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
	// The batch records what file returns for the channels it moves (see
	// [Filer] and [Filing]). file runs under the same lock or transaction
	// as the move, and not at all when nothing moves. An invalid filing, or
	// an error from file, moves nothing and is returned.
	//
	// No list, Remove or Delete reaches a tombstoned channel. Only the
	// batch methods do: [Store.Tombstones] lists batches,
	// [Store.Untombstone] puts one back and [Store.Purge] erases one.
	Tombstone(ctx context.Context, id string, face entity.Face, at time.Time, file Filer) (TombstoneHandle, error)

	// ConfirmTombstone makes the batch h names final (see
	// [Filing.Confirmed]): it clears the pending mark, so [Store.Purge] may
	// erase it, and turns an orphan batch into a detach batch. A batch that
	// is already final is left as it is. A batch already restored or
	// purged, or one that recorded no filing, is [ErrNotFound].
	ConfirmTombstone(ctx context.Context, h TombstoneHandle) error

	// ConfirmTombstones confirms, as ConfirmTombstone does, every batch of
	// anchor: every batch of the id for an empty face, the face's batches
	// otherwise. A new entity reusing an id calls it, so no batch of the
	// old entity can be restored onto the new one. An anchor without
	// batches is not an error.
	ConfirmTombstones(ctx context.Context, anchor Anchor) error

	// Tombstones returns the batches filed before the given time, oldest
	// first. A batch holding no channel is left out: it has nothing to
	// purge.
	Tombstones(ctx context.Context, before time.Time) ([]TombstoneBatch, error)

	// DueTombstones returns the confirmed batches with something due at or
	// before the given time (see [Filing.DueAt]), oldest first. Batches
	// that recorded no purge time are left out, as are those holding no
	// channel. It reads an index, so the purge poll can call it often.
	DueTombstones(ctx context.Context, at time.Time) ([]TombstoneBatch, error)

	// Purge erases channels and messages of exactly the batch h names:
	// the channels of the given types, or every channel when types is
	// empty. A partial purge leaves the rest of the batch where it is,
	// drops the erased types from the filing's TypeAfter, and becomes a
	// whole purge when it erases the last channel. A whole purge also ends
	// the batch: its filed audit trail is kept, and the records
	// [PurgeAudit] returns are appended to it, never to the live trail of
	// the id, since that id may belong to a new entity by now. A partial
	// purge appends its records the same way.
	//
	// A pending batch is [ErrConflict]. A batch already restored or
	// purged is [ErrNotFound]. The zero handle is a no-op.
	Purge(ctx context.Context, h TombstoneHandle, types []string, by Author, at time.Time) error

	// DropTypeRecord removes the record of typ (see [TypeRecord]) when no
	// channel of typ exists, live or tombstoned; otherwise it is
	// [ErrTypeInUse] and nothing changes. A type without a record is not an
	// error. The check and the removal are one step, so a channel created
	// concurrently either blocks the drop or is created after it.
	DropTypeRecord(ctx context.Context, typ string) error

	// Untombstone puts back exactly the batch h names, with the audit trail
	// a whole-entity tombstone filed away, and records the restore in the
	// trail. It is how a caller undoes a tombstone it took before a write
	// that then failed.
	//
	// When live channels sit on the batch's anchor (any channel of the id
	// for a whole-entity batch, any channel of the face for a face batch),
	// it refuses with [*OccupiedError] and nothing changes. A batch whose
	// filing is final (see [Filing.Restorable]) is [ErrConflict] and
	// nothing changes. A batch already restored or purged, or never filed,
	// is [ErrNotFound]. The zero handle is a no-op.
	//
	// resume is called, in the same step, once for each restored channel
	// that is open, and its result stored (see [ScheduleFunc]). The
	// restore of an open channel resumes its session timers. Nil:
	// accepted; the schedules are kept as they were.
	Untombstone(ctx context.Context, h TombstoneHandle, at time.Time, resume ScheduleFunc) error

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

	// ListByLifecycle returns the channels of type typ in state that
	// entered it before the given time (see [Channel.StateSince]), for the
	// close and archive sweeps. typ is required.
	ListByLifecycle(ctx context.Context, typ string, state State, before time.Time) ([]Channel, error)

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
	// empty map. A record written before cardinality was kept reads with
	// [CardinalityUnknown]. Compare it with the configuration through
	// [TypeRecordChanges].
	TypeRecords(ctx context.Context) (map[string]TypeRecord, error)

	// RecordTypes records each given type that the store holds no record
	// for, and adds the given refs to the record of a type it does hold.
	// It never replaces an anchor mode or cardinality: a record that
	// differs from the held one in either, or whose Plain names a held
	// ref, is [ErrTypeShapeConflict] (see [MergeTypeRecord]). A held
	// [CardinalityUnknown] is the exception: it matches, and the call sets
	// it to the given cardinality. It writes nothing when nothing is new. Types not given keep theirs. An invalid
	// type is [ErrInvalidKey], an unknown mode or cardinality
	// [ErrInvalidMessage]; on any error nothing is recorded.
	RecordTypes(ctx context.Context, records map[string]TypeRecord) error
}
