// Package sqlchannels is the one implementation of [channels.Store] over a
// SQL database. pgchannels and sqlitechannels adapt their driver to [DB]
// and pick a [Dialect]; the queries, the locking order and the erasure
// steps live here once, so the two database backends cannot drift apart.
//
// # Schema
//
// Five tables, created by the pgstore and sqlitedb migrations:
//
//   - channels: one row per channel, keyed by (anchor_id, anchor_face, type,
//     name, tomb). tomb is empty for a live channel and holds the tombstone
//     stamp of a detached one, so a detached channel never collides with a
//     live one of the same key.
//   - channel_messages: one row per message. The seq, rev, thread link and
//     pending time are columns; everything else is the JSON doc column. A
//     removal replaces the doc with the removal record, so the content is
//     gone from the row, not only hidden.
//   - channel_audit: the audit trail, one JSON record per row, with the same
//     tomb column as channels.
//   - channel_types: the anchor mode and cardinality recorded for each
//     channel type, and channel_type_refs its ref properties. An empty
//     cardinality is a row from before the column existed
//     ([channels.CardinalityUnknown]).
//
// # Erasure
//
// Remove and Edit overwrite the row, which is necessary but not sufficient
// for erasure on disk. The old bytes can survive in freed SQLite pages and
// WAL frames unless secure_delete is on, and in PostgreSQL dead tuples until
// VACUUM, in the WAL, and in backups. This is the same caveat as version
// purge; the conformance suite checks the live rows only.
//
// Times are BIGINT microseconds since the epoch, 0 for none, so both
// databases compare and order them alike. JSON is TEXT on both: JSONB would
// reject a "\u0000" escape that SQLite accepts, and nothing queries inside
// the documents.
//
// # Locking
//
// Every mutation runs in one transaction. The dialect's entity lock, when it
// has one, is taken first and serializes the writes of one entity across
// processes; Rekey takes the locks of both ids in a fixed order. SQLite needs
// no entity lock, because its write transaction already excludes every other
// writer.
package sqlchannels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ErrNoRows is what a [Row] returns from Scan when the query matched
// nothing. Adapters translate their driver's own sentinel to it.
var ErrNoRows = errors.New("sqlchannels: no rows")

// Querier runs statements. Queries use `?` placeholders; an adapter whose
// driver numbers them rewrites the query.
type Querier interface {
	Exec(ctx context.Context, query string, args ...any) (rowsAffected int64, err error)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row
}

// Rows is a result set.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Row is a single-row result.
type Row interface {
	Scan(dest ...any) error
}

// DB is a database handle that runs transactions.
type DB interface {
	Querier
	// Transact runs fn in one transaction, committing when fn returns nil and
	// rolling back otherwise.
	Transact(ctx context.Context, fn func(q Querier) error) error
}

// Dialect holds what differs between the databases.
type Dialect struct {
	// Name prefixes error messages, e.g. "pgchannels".
	Name string
	// LockEntity is a statement with one `?` argument that takes a
	// transaction-scoped lock on an entity's channels. Empty when the
	// transaction itself excludes every other writer.
	LockEntity string
}

// Store is the SQL backend.
//
// 20 -> 21 (TKT-ZU7P13): Untombstone. 21 -> 25 (TKT-964SIS):
// ConfirmTombstone, Tombstones, Purge and DropTypeRecord. The exported
// methods are the mandated [channels.Store] interface, so this is a
// required-interface exception rather than a ratchet target. Helpers stay
// free functions.
//
//plimsoll:max-exported-methods=27
type Store struct {
	db DB
	d  Dialect
}

var _ channels.Store = (*Store)(nil)

// New returns a store over db.
//
// Nil: db rejected, so a wiring mistake fails here rather than at the first
// message.
func New(db DB, d Dialect) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("%s: New requires a database handle", d.Name)
	}
	return &Store{db: db, d: d}, nil
}

// --- encoding ---

func micros(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMicro()
}

func fromMicros(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.UnixMicro(v).UTC()
}

// doc is the JSON column of a message: everything but the columns.
type doc struct {
	Type        string            `json:"type,omitempty"`
	Author      channels.Author   `json:"author,omitzero"`
	At          time.Time         `json:"at,omitzero"`
	Props       map[string]any    `json:"props,omitempty"`
	EditedAt    time.Time         `json:"edited_at,omitzero"`
	Removed     *channels.Removal `json:"removed,omitempty"`
	Action      *channels.Action  `json:"action,omitempty"`
	SchemaHash  string            `json:"schema_hash,omitempty"`
	TriggeredBy string            `json:"triggered_by,omitempty"`
}

func docOf(m channels.Message) string {
	return mustJSON(doc{
		Type: m.Type, Author: m.Author, At: m.At, Props: m.Props, EditedAt: m.EditedAt,
		Removed: m.Removed, Action: m.Action, SchemaHash: m.SchemaHash, TriggeredBy: m.TriggeredBy,
	})
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil { // coverage-ignore: invariant: values were encoded once at the boundary
		panic(fmt.Sprintf("sqlchannels: encode: %v", err))
	}
	return string(b)
}

const msgCols = `seq, rev, reply_to, reply_count, doc`

func scanMessage(r interface{ Scan(...any) error }) (channels.Message, error) {
	var m channels.Message
	var raw string
	if err := r.Scan(&m.Seq, &m.Rev, &m.ReplyTo, &m.ReplyCount, &raw); err != nil {
		return m, err
	}
	var d doc
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return m, fmt.Errorf("message %d has a damaged doc: %w", m.Seq, err)
	}
	m.Type, m.Author, m.Props, m.SchemaHash, m.TriggeredBy = d.Type, d.Author, d.Props, d.SchemaHash, d.TriggeredBy
	m.At, m.EditedAt = utc(d.At), utc(d.EditedAt)
	m.Removed, m.Action = d.Removed, d.Action
	if m.Removed != nil {
		m.Removed.At = utc(m.Removed.At)
	}
	if m.Action != nil {
		m.Action.At, m.Action.PendingAt = utc(m.Action.At), utc(m.Action.PendingAt)
	}
	return m, nil
}

func utc(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC()
}

const chanCols = `id, anchor_id, anchor_face, type, name, incarnation, state, title, created_by,
	created_at, closed_at, archived_at, reopened_at, last_seq, rev`

// openSinceSQL is when an open channel entered its state, as
// [channels.Channel.StateSince] reads it.
const openSinceSQL = `(CASE WHEN reopened_at > 0 THEN reopened_at ELSE created_at END)`

// row is a channel row: the channel and its surrogate id.
type row struct {
	id int64
	c  channels.Channel
}

func scanChannel(r interface{ Scan(...any) error }) (row, error) {
	var out row
	var face, by string
	var created, closed, archived, reopened int64
	c := &out.c
	if err := r.Scan(&out.id, &c.Key.ID, &face, &c.Key.Type, &c.Key.Name, &c.Incarnation, &c.State, &c.Title, &by,
		&created, &closed, &archived, &reopened, &c.LastSeq, &c.Rev); err != nil {
		return out, err
	}
	c.Key.Face = entity.Face(face)
	if err := json.Unmarshal([]byte(by), &c.CreatedBy); err != nil {
		return out, fmt.Errorf("channel %s has a damaged created_by: %w", c.Key, err)
	}
	c.CreatedAt, c.ClosedAt, c.ArchivedAt = fromMicros(created), fromMicros(closed), fromMicros(archived)
	c.ReopenedAt = fromMicros(reopened)
	return out, nil
}

func (s *Store) wrap(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*channels.OccupiedError](err); ok {
		return err
	}
	for _, sentinel := range []error{
		channels.ErrNotFound, channels.ErrInvalidKey, channels.ErrInvalidMessage, channels.ErrTooLarge,
		channels.ErrTooManyChannels, channels.ErrConflict, channels.ErrRemoved, channels.ErrInvalidReply,
		channels.ErrInvalidPage, channels.ErrRekeyConflict, channels.ErrStaleAnchor, channels.ErrTypeShapeConflict,
		channels.ErrTypeInUse,
	} {
		if errors.Is(err, sentinel) {
			return err
		}
	}
	return fmt.Errorf("%s: %s: %w", s.d.Name, fmt.Sprintf(format, args...), err)
}

// --- transactions ---

// write runs fn in a transaction holding the entity locks of ids.
func (s *Store) write(ctx context.Context, ids []string, fn func(q Querier) error) error {
	return s.db.Transact(ctx, func(q Querier) error {
		if s.d.LockEntity != "" {
			if len(ids) == 2 && ids[1] < ids[0] {
				ids = []string{ids[1], ids[0]}
			}
			for _, id := range ids {
				if _, err := q.Exec(ctx, s.d.LockEntity, id); err != nil {
					return fmt.Errorf("lock %s: %w", id, err)
				}
			}
		}
		return fn(q)
	})
}

const keyWhere = `anchor_id = ? AND anchor_face = ? AND type = ? AND name = ? AND tomb = ''`

func keyArgs(k channels.Key) []any { return []any{k.ID, string(k.Face), k.Type, k.Name} }

func (s *Store) channelRow(ctx context.Context, q Querier, k channels.Key) (row, error) {
	r, err := scanChannel(q.QueryRow(ctx, `SELECT `+chanCols+` FROM channels WHERE `+keyWhere, keyArgs(k)...))
	if errors.Is(err, ErrNoRows) {
		return r, fmt.Errorf("%w: channel %s", channels.ErrNotFound, k)
	}
	return r, err
}

func (s *Store) message(ctx context.Context, q Querier, chanID, seq int64, k channels.Key) (channels.Message, error) {
	m, err := scanMessage(q.QueryRow(ctx,
		`SELECT `+msgCols+` FROM channel_messages WHERE channel_id = ? AND seq = ?`, chanID, seq))
	if errors.Is(err, ErrNoRows) {
		return m, fmt.Errorf("%w: message %d in %s", channels.ErrNotFound, seq, k)
	}
	return m, err
}

func (s *Store) audit(ctx context.Context, q Querier, id string, rec channels.AuditRecord) error {
	_, err := q.Exec(ctx, `INSERT INTO channel_audit (anchor_id, tomb, record) VALUES (?, '', ?)`, id, mustJSON(rec))
	return err
}

func (s *Store) bumpChannel(ctx context.Context, q Querier, id, lastSeq, rev int64) error {
	_, err := q.Exec(ctx, `UPDATE channels SET last_seq = ?, rev = ? WHERE id = ?`, lastSeq, rev, id)
	return err
}

// --- mutations ---

// Append implements [channels.Store].
func (s *Store) Append(ctx context.Context, k channels.Key, nm channels.NewMessage) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	nm, _, err := channels.PrepareMessage(nm)
	if err != nil {
		return channels.Message{}, err
	}
	var out channels.Message
	werr := s.write(ctx, []string{k.ID}, func(q Querier) error {
		r, created, err := s.appendTarget(ctx, q, k, nm)
		if err != nil {
			return err
		}
		err = s.checkParent(ctx, q, r.id, k, nm.ReplyTo)
		if err != nil {
			return err
		}
		rev := r.c.Rev + 1
		out = channels.Message{
			Seq: r.c.LastSeq + 1, Rev: rev, Type: nm.Type, Author: nm.Author, At: nm.At, Props: nm.Props,
			ReplyTo: nm.ReplyTo, SchemaHash: nm.SchemaHash, TriggeredBy: nm.TriggeredBy,
		}
		if _, err := q.Exec(ctx, `INSERT INTO channel_messages
			(channel_id, seq, rev, reply_to, reply_count, pending_at, doc) VALUES (?, ?, ?, ?, 0, 0, ?)`,
			r.id, out.Seq, out.Rev, out.ReplyTo, docOf(out)); err != nil {
			return err
		}
		if nm.ReplyTo > 0 {
			rev++
			if _, err := q.Exec(ctx, `UPDATE channel_messages SET reply_count = reply_count + 1, rev = ?
				WHERE channel_id = ? AND seq = ?`, rev, r.id, nm.ReplyTo); err != nil {
				return err
			}
		}
		if err := s.bumpChannel(ctx, q, r.id, out.Seq, rev); err != nil {
			return err
		}
		if created {
			rec := channels.KeyAudit(k, channels.AuditCreate, nm.At)
			rec.By = nm.Author
			return s.audit(ctx, q, k.ID, rec)
		}
		return nil
	})
	return out, s.wrap(werr, "append to %s", k)
}

// appendTarget returns the channel an append goes to, creating an implicit
// channel on its first message. created reports that it did.
func (s *Store) appendTarget(
	ctx context.Context, q Querier, k channels.Key, nm channels.NewMessage,
) (r row, created bool, err error) {
	r, err = s.channelRow(ctx, q, k)
	if errors.Is(err, channels.ErrNotFound) && k.Name == "" {
		r, err = s.insertChannel(ctx, q, k, channels.NewChannel{
			Incarnation: nm.Incarnation, By: nm.Author, At: nm.At,
		})
		return r, err == nil, err
	}
	if err == nil {
		err = channels.CheckAppendIncarnation(k, r.c.Incarnation, nm.Incarnation)
	}
	return r, false, err
}

// checkParent refuses a reply to a missing, removed or reply message. A
// replyTo of zero is not a reply and passes.
func (s *Store) checkParent(ctx context.Context, q Querier, channelID int64, k channels.Key, replyTo int64) error {
	if replyTo == 0 {
		return nil
	}
	parent, err := s.message(ctx, q, channelID, replyTo, k)
	if errors.Is(err, channels.ErrNotFound) {
		return fmt.Errorf("%w: message %d does not exist", channels.ErrInvalidReply, replyTo)
	}
	if err != nil {
		return err
	}
	if parent.Removed != nil || parent.ReplyTo != 0 {
		return fmt.Errorf("%w: message %d is removed or a reply", channels.ErrInvalidReply, replyTo)
	}
	return nil
}

// insertChannel inserts the channel k. Only the title, incarnation, author
// and time of nc are read; k carries the type. It takes the type's lock
// (see typeLock) after the caller's entity lock, so a create and a
// DropTypeRecord of the type serialize.
func (s *Store) insertChannel(ctx context.Context, q Querier, k channels.Key, nc channels.NewChannel) (row, error) {
	if s.d.LockEntity != "" {
		if _, err := q.Exec(ctx, s.d.LockEntity, typeLock(k.Type)); err != nil {
			return row{}, fmt.Errorf("lock type %s: %w", k.Type, err)
		}
	}
	_, err := q.Exec(ctx, `INSERT INTO channels
		(anchor_id, anchor_face, type, name, tomb, incarnation, state, title, created_by, created_at, closed_at,
		 archived_at, last_seq, rev)
		VALUES (?, ?, ?, ?, '', ?, ?, ?, ?, ?, 0, 0, 0, 0)`,
		k.ID, string(k.Face), k.Type, k.Name, nc.Incarnation, string(channels.StateOpen), nc.Title,
		mustJSON(nc.By), micros(nc.At))
	if err != nil {
		return row{}, err
	}
	return s.channelRow(ctx, q, k)
}

// update loads message seq of k in a write transaction and hands it to
// change, which edits it in place and may return extra statements' errors.
// The message's rev and the channel's rev move together.
func (s *Store) update(
	ctx context.Context, k channels.Key, seq int64, verb string,
	change func(q Querier, r row, m *channels.Message) error,
) (channels.Message, error) {
	var out channels.Message
	err := s.write(ctx, []string{k.ID}, func(q Querier) error {
		r, err := s.channelRow(ctx, q, k)
		if err != nil {
			return err
		}
		m, err := s.message(ctx, q, r.id, seq, k)
		if err != nil {
			return err
		}
		if m.Removed != nil {
			return fmt.Errorf("%w: message %d in %s", channels.ErrRemoved, seq, k)
		}
		m.Rev = r.c.Rev + 1
		if err := change(q, r, &m); err != nil {
			return err
		}
		pending := int64(0)
		if m.Action != nil && m.Action.Status == channels.ActionPending {
			pending = micros(m.Action.PendingAt)
		}
		if _, err := q.Exec(ctx, `UPDATE channel_messages SET rev = ?, pending_at = ?, doc = ?
			WHERE channel_id = ? AND seq = ?`, m.Rev, pending, docOf(m), r.id, seq); err != nil {
			return err
		}
		out = m
		return s.bumpChannel(ctx, q, r.id, r.c.LastSeq, m.Rev)
	})
	return out, s.wrap(err, "%s message %d in %s", verb, seq, k)
}

// Edit implements [channels.Store].
func (s *Store) Edit(
	ctx context.Context, k channels.Key, seq int64, props map[string]any, at time.Time,
) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	props, _, err := channels.PrepareProps(props)
	if err != nil {
		return channels.Message{}, err
	}
	at = channels.NormTime(at)
	return s.update(ctx, k, seq, "edit", func(_ Querier, _ row, m *channels.Message) error {
		m.Props, m.EditedAt = props, at
		return nil
	})
}

// Remove implements [channels.Store].
func (s *Store) Remove(ctx context.Context, k channels.Key, seq int64, rm channels.Removal) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	rm, err := channels.PrepareRemoval(rm)
	if err != nil {
		return channels.Message{}, err
	}
	return s.update(ctx, k, seq, "remove", func(q Querier, _ row, m *channels.Message) error {
		*m = channels.Message{Seq: m.Seq, Rev: m.Rev, ReplyTo: m.ReplyTo, ReplyCount: m.ReplyCount, Removed: &rm}
		rec := channels.KeyAudit(k, channels.AuditRemove, rm.At)
		rec.Seq, rec.By = seq, rm.By
		return s.audit(ctx, q, k.ID, rec)
	})
}

// SetAction implements [channels.Store].
func (s *Store) SetAction(
	ctx context.Context, k channels.Key, seq int64, name string, expect channels.ActionStatus, next channels.Action,
) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	next, err := channels.PrepareAction(name, next)
	if err != nil {
		return channels.Message{}, err
	}
	if err := channels.CheckRelease(expect, next); err != nil {
		return channels.Message{}, err
	}
	return s.update(ctx, k, seq, "act on", func(q Querier, _ row, m *channels.Message) error {
		if err := channels.CheckActionCAS(m.Action, name, expect); err != nil {
			return err
		}
		if next.Status == channels.ActionNone {
			m.Action = nil
		} else {
			a := next
			m.Action = &a
		}
		rec, ok := channels.ActionAudit(k, seq, name, expect, next)
		if !ok {
			return nil
		}
		return s.audit(ctx, q, k.ID, rec)
	})
}

// CreateMany implements [channels.Store].
func (s *Store) CreateMany(
	ctx context.Context, a channels.Anchor, nc channels.NewChannel,
) (channels.Channel, error) {
	nc, err := channels.PrepareChannel(a, nc)
	if err != nil {
		return channels.Channel{}, err
	}
	typ := nc.Type
	var out channels.Channel
	werr := s.write(ctx, []string{a.ID}, func(q Querier) error {
		var n int
		if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM channels
			WHERE anchor_id = ? AND anchor_face = ? AND type = ? AND name <> '' AND tomb = ''`,
			a.ID, string(a.Face), typ).Scan(&n); err != nil {
			return err
		}
		if n >= nc.Cap() {
			return fmt.Errorf("%w: %s holds %d %s channels", channels.ErrTooManyChannels, a.ID, n, typ)
		}
		k := channels.Key{Anchor: a, Type: typ, Name: channels.NewName()}
		r, err := s.insertChannel(ctx, q, k, nc)
		if err != nil {
			return err
		}
		out = r.c
		rec := channels.KeyAudit(k, channels.AuditCreate, nc.At)
		rec.By = nc.By
		return s.audit(ctx, q, a.ID, rec)
	})
	return out, s.wrap(werr, "create %s channel on %s", typ, a.ID)
}

// SetLifecycle implements [channels.Store].
func (s *Store) SetLifecycle(
	ctx context.Context, k channels.Key, expect channels.State, t channels.Transition,
) (channels.Channel, error) {
	if err := k.Validate(); err != nil {
		return channels.Channel{}, err
	}
	t, err := channels.PrepareTransition(expect, t)
	if err != nil {
		return channels.Channel{}, err
	}
	var out channels.Channel
	werr := s.write(ctx, []string{k.ID}, func(q Querier) error {
		r, err := s.channelRow(ctx, q, k)
		if err != nil {
			return err
		}
		if r.c.State != expect {
			return fmt.Errorf("%w: %s is %s, expected %s", channels.ErrConflict, k, r.c.State, expect)
		}
		c := r.c
		channels.ApplyTransition(&c, t)
		c.Rev++
		if _, err := q.Exec(ctx, `UPDATE channels SET state = ?, closed_at = ?, archived_at = ?, reopened_at = ?,
			rev = ? WHERE id = ?`, string(c.State), micros(c.ClosedAt), micros(c.ArchivedAt), micros(c.ReopenedAt),
			c.Rev, r.id); err != nil {
			return err
		}
		out = c
		rec := channels.KeyAudit(k, channels.AuditLifecycle, t.At)
		rec.By, rec.From, rec.To, rec.TriggeredBy = t.By, string(expect), string(t.To), t.TriggeredBy
		return s.audit(ctx, q, k.ID, rec)
	})
	return out, s.wrap(werr, "move %s", k)
}

// Delete implements [channels.Store].
func (s *Store) Delete(ctx context.Context, k channels.Key, by channels.Author, at time.Time) error {
	if err := k.Validate(); err != nil {
		return err
	}
	err := s.write(ctx, []string{k.ID}, func(q Querier) error {
		r, err := s.channelRow(ctx, q, k)
		if err != nil {
			return err
		}
		// Messages first: the foreign key cascades too, but SQLite only
		// enforces it with the foreign_keys pragma, and the erasure must not
		// depend on a connection setting.
		if _, err := q.Exec(ctx, `DELETE FROM channel_messages WHERE channel_id = ?`, r.id); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM channels WHERE id = ?`, r.id); err != nil {
			return err
		}
		rec := channels.KeyAudit(k, channels.AuditDelete, channels.NormTime(at))
		rec.By = by
		return s.audit(ctx, q, k.ID, rec)
	})
	return s.wrap(err, "delete %s", k)
}

// Rekey implements [channels.Store].
func (s *Store) Rekey(ctx context.Context, oldID, newID string, at time.Time) error {
	if err := channels.CheckRekey(oldID, newID); err != nil {
		return err
	}
	err := s.write(ctx, []string{oldID, newID}, func(q Querier) error {
		var n int
		if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM channels WHERE anchor_id = ? AND tomb = ''`,
			newID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%w: %s", channels.ErrRekeyConflict, newID)
		}
		moved, err := q.Exec(ctx, `UPDATE channels SET anchor_id = ? WHERE anchor_id = ? AND tomb = ''`, newID, oldID)
		if err != nil {
			return err
		}
		trail, err := q.Exec(ctx, `UPDATE channel_audit SET anchor_id = ? WHERE anchor_id = ? AND tomb = ''`,
			newID, oldID)
		if err != nil {
			return err
		}
		if moved == 0 && trail == 0 {
			return nil
		}
		return s.audit(ctx, q, newID, channels.AuditRecord{
			At: channels.NormTime(at), Op: channels.AuditRekey, From: oldID, To: newID,
		})
	})
	return s.wrap(err, "rekey %s to %s", oldID, newID)
}

// Tombstone implements [channels.Store]. The batch's rows keep their
// place and get the stamp in their tomb column; its filing is a row of
// channel_tombstones.
func (s *Store) Tombstone(
	ctx context.Context, id string, face entity.Face, at time.Time, file channels.Filer,
) (channels.TombstoneHandle, error) {
	a := channels.Anchor{ID: id, Face: face}
	if err := a.Validate(); err != nil {
		return channels.TombstoneHandle{}, err
	}
	h := channels.NewTombstoneHandle(a, channels.TombstoneStamp(at))
	rec := channels.AuditRecord{At: channels.NormTime(at), Op: channels.AuditTombstone, Face: face}
	filed := false
	var fileErr error
	err := s.write(ctx, []string{id}, func(q Querier) error {
		// The filing is computed from the rows this transaction moves,
		// read under the same write lock (see channels.Filer).
		where, args := `AND anchor_id = ?`, []any{id}
		if face != "" {
			where, args = where+` AND anchor_face = ?`, append(args, string(face))
		}
		batch, err := queryChannels(ctx, q, where, args...)
		if err != nil {
			return err
		}
		var f channels.Filing
		if len(batch) > 0 {
			if f, fileErr = channels.FileBatch(file, batch); fileErr != nil {
				return fileErr
			}
		}
		if face != "" {
			moved, err := q.Exec(ctx, `UPDATE channels SET tomb = ?
				WHERE anchor_id = ? AND anchor_face = ? AND tomb = ''`, h.Stamp(), id, string(face))
			if err != nil || moved == 0 {
				return err
			}
			filed = true
			if err := insertFiling(ctx, q, h, f); err != nil {
				return err
			}
			return s.audit(ctx, q, id, rec)
		}
		moved, err := q.Exec(ctx, `UPDATE channels SET tomb = ? WHERE anchor_id = ? AND tomb = ''`,
			h.Stamp(), id)
		if err != nil {
			return err
		}
		filed = moved > 0
		if filed {
			if fileErr := insertFiling(ctx, q, h, f); fileErr != nil {
				return fileErr
			}
		}
		var n int
		if cErr := q.QueryRow(ctx, `SELECT COUNT(*) FROM channel_audit WHERE anchor_id = ? AND tomb = ''`,
			id).Scan(&n); cErr != nil || n == 0 {
			return cErr
		}
		filed = true
		if aErr := s.audit(ctx, q, id, rec); aErr != nil {
			return aErr
		}
		_, err = q.Exec(ctx, `UPDATE channel_audit SET tomb = ? WHERE anchor_id = ? AND tomb = ''`, h.Stamp(), id)
		return err
	})
	if fileErr != nil {
		return channels.TombstoneHandle{}, fileErr
	}
	if err != nil || !filed {
		return channels.TombstoneHandle{}, s.wrap(err, "tombstone %s", entity.FormatStateRef(id, face))
	}
	return h, nil
}

// Untombstone implements [channels.Store]. A stamp is unique to one
// Tombstone call, so the rows carrying it are exactly the batch.
func (s *Store) Untombstone(ctx context.Context, h channels.TombstoneHandle, at time.Time) error {
	if h.IsZero() {
		return nil
	}
	if err := h.Validate(); err != nil {
		return err
	}
	a := h.Anchor()
	err := s.write(ctx, []string{a.ID}, func(q Querier) error {
		var filedChannels, filedAudit, live int
		if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM channels WHERE anchor_id = ? AND tomb = ?`,
			a.ID, h.Stamp()).Scan(&filedChannels); err != nil {
			return err
		}
		if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM channel_audit WHERE anchor_id = ? AND tomb = ?`,
			a.ID, h.Stamp()).Scan(&filedAudit); err != nil {
			return err
		}
		if filedChannels == 0 && filedAudit == 0 {
			return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
		}
		f, ok, err := readFiling(ctx, q, h)
		if err != nil {
			return err
		}
		if ok && !f.filing.Restorable() {
			return fmt.Errorf("%w: tombstone %s is final", channels.ErrConflict, h)
		}
		liveQuery, liveArgs := `SELECT COUNT(*) FROM channels WHERE anchor_id = ? AND tomb = ''`, []any{a.ID}
		if a.Face != "" {
			liveQuery += ` AND anchor_face = ?`
			liveArgs = append(liveArgs, string(a.Face))
		}
		if err := q.QueryRow(ctx, liveQuery, liveArgs...).Scan(&live); err != nil {
			return err
		}
		if live > 0 {
			return &channels.OccupiedError{Anchor: a}
		}
		if _, err := q.Exec(ctx, `UPDATE channels SET tomb = '' WHERE anchor_id = ? AND tomb = ?`,
			a.ID, h.Stamp()); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `UPDATE channel_audit SET tomb = '' WHERE anchor_id = ? AND tomb = ?`,
			a.ID, h.Stamp()); err != nil {
			return err
		}
		if err := deleteFiling(ctx, q, h); err != nil {
			return err
		}
		return s.audit(ctx, q, a.ID, channels.AuditRecord{
			At: channels.NormTime(at), Op: channels.AuditUntombstone, Face: a.Face,
		})
	})
	return s.wrap(err, "untombstone %s", entity.FormatStateRef(a.ID, a.Face))
}

// --- reads ---

// Head implements [channels.Store].
func (s *Store) Head(ctx context.Context, k channels.Key) (channels.Channel, error) {
	if err := k.Validate(); err != nil {
		return channels.Channel{}, err
	}
	r, err := s.channelRow(ctx, s.db, k)
	return r.c, s.wrap(err, "read %s", k)
}

// Get implements [channels.Store].
func (s *Store) Get(ctx context.Context, k channels.Key, seq int64) (channels.Message, error) {
	if err := k.Validate(); err != nil {
		return channels.Message{}, err
	}
	r, err := s.channelRow(ctx, s.db, k)
	if err != nil {
		return channels.Message{}, s.wrap(err, "read %s", k)
	}
	m, err := s.message(ctx, s.db, r.id, seq, k)
	return m, s.wrap(err, "read message %d in %s", seq, k)
}

// Messages implements [channels.Store].
func (s *Store) Messages(ctx context.Context, k channels.Key, p channels.Page) ([]channels.Message, error) {
	if err := k.Validate(); err != nil {
		return nil, err
	}
	p, err := p.Normalize()
	if err != nil {
		return nil, err
	}
	r, err := s.channelRow(ctx, s.db, k)
	if err != nil {
		return nil, s.wrap(err, "read %s", k)
	}
	base := `SELECT ` + msgCols + ` FROM channel_messages WHERE channel_id = ?`
	var query string
	args := []any{r.id}
	reverse := false
	switch {
	case p.AfterRev > 0:
		query, args = base+` AND rev > ? ORDER BY rev LIMIT ?`, append(args, p.AfterRev, p.Limit)
	case p.AfterSeq > 0:
		query, args = base+` AND seq > ? ORDER BY seq LIMIT ?`, append(args, p.AfterSeq, p.Limit)
	case p.BeforeSeq > 0:
		query, args, reverse = base+` AND seq < ? ORDER BY seq DESC LIMIT ?`, append(args, p.BeforeSeq, p.Limit), true
	default:
		query, args, reverse = base+` ORDER BY seq DESC LIMIT ?`, append(args, p.Limit), true
	}
	out, err := collect(ctx, s.db, query, args, scanMessage)
	if err != nil {
		return nil, s.wrap(err, "page %s", k)
	}
	if reverse {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}

// collect runs a query and scans every row. The result is non-nil.
func collect[T any](ctx context.Context, q Querier, query string, args []any,
	scan func(r interface{ Scan(...any) error }) (T, error),
) ([]T, error) {
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) channels(ctx context.Context, where string, args ...any) ([]channels.Channel, error) {
	return queryChannels(ctx, s.db, where, args...)
}

// queryChannels lists live channels over q, so a transaction can read the
// rows it is about to change.
func queryChannels(ctx context.Context, q Querier, where string, args ...any) ([]channels.Channel, error) {
	rows, err := collect(ctx, q, `SELECT `+chanCols+` FROM channels WHERE tomb = '' `+where+
		` ORDER BY anchor_id, anchor_face, type, name`, args, scanChannel)
	if err != nil {
		return nil, err
	}
	out := make([]channels.Channel, len(rows))
	for i, r := range rows {
		out[i] = r.c
	}
	return out, nil
}

// List implements [channels.Store].
func (s *Store) List(
	ctx context.Context, a channels.Anchor, typ string, state channels.State,
) ([]channels.Channel, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	where := `AND anchor_id = ? AND anchor_face = ?`
	args := []any{a.ID, string(a.Face)}
	if typ != "" {
		where, args = where+` AND type = ?`, append(args, typ)
	}
	if state != "" {
		where, args = where+` AND state = ?`, append(args, string(state))
	}
	out, err := s.channels(ctx, where, args...)
	return out, s.wrap(err, "list %s", a.ID)
}

// ListEntity implements [channels.Store].
func (s *Store) ListEntity(ctx context.Context, id string) ([]channels.Channel, error) {
	if err := (channels.Anchor{ID: id}).Validate(); err != nil {
		return nil, err
	}
	out, err := s.channels(ctx, `AND anchor_id = ?`, id)
	return out, s.wrap(err, "list %s", id)
}

// ListByLifecycle implements [channels.Store]. channels_type_state_idx
// narrows the scan to the type and state.
func (s *Store) ListByLifecycle(
	ctx context.Context, typ string, state channels.State, before time.Time,
) ([]channels.Channel, error) {
	if !channels.ValidName(typ) {
		return nil, fmt.Errorf("%w: channel type %q", channels.ErrInvalidKey, typ)
	}
	col := map[channels.State]string{
		channels.StateOpen: openSinceSQL, channels.StateClosed: "closed_at", channels.StateArchived: "archived_at",
	}[state]
	if col == "" {
		return nil, fmt.Errorf("%w: state %q", channels.ErrInvalidMessage, state)
	}
	out, err := s.channels(ctx, `AND type = ? AND state = ? AND `+col+` < ?`,
		typ, string(state), micros(channels.NormTime(before)))
	return out, s.wrap(err, "list %s %s channels", state, typ)
}

// StalePending implements [channels.Store].
func (s *Store) StalePending(ctx context.Context, before time.Time) ([]channels.PendingAction, error) {
	out, err := collect(ctx, s.db, `SELECT c.anchor_id, c.anchor_face, c.type, c.name, m.seq, m.doc
		FROM channel_messages m JOIN channels c ON c.id = m.channel_id
		WHERE c.tomb = '' AND m.pending_at > 0 AND m.pending_at < ?
		ORDER BY c.anchor_id, c.anchor_face, c.type, c.name, m.seq`,
		[]any{micros(channels.NormTime(before))},
		func(r interface{ Scan(...any) error }) (channels.PendingAction, error) {
			var p channels.PendingAction
			var face, raw string
			if err := r.Scan(&p.Key.ID, &face, &p.Key.Type, &p.Key.Name, &p.Seq, &raw); err != nil {
				return p, err
			}
			p.Key.Face = entity.Face(face)
			var d doc
			if err := json.Unmarshal([]byte(raw), &d); err != nil || d.Action == nil {
				return p, fmt.Errorf("pending message %d in %s has a damaged doc", p.Seq, p.Key)
			}
			p.Name, p.PendingAt = d.Action.Name, utc(d.Action.PendingAt)
			return p, nil
		})
	return out, s.wrap(err, "list pending actions")
}

// Audit implements [channels.Store].
func (s *Store) Audit(ctx context.Context, id string) ([]channels.AuditRecord, error) {
	if err := (channels.Anchor{ID: id}).Validate(); err != nil {
		return nil, err
	}
	out, err := collect(ctx, s.db, `SELECT record FROM channel_audit WHERE anchor_id = ? AND tomb = '' ORDER BY id`,
		[]any{id}, func(r interface{ Scan(...any) error }) (channels.AuditRecord, error) {
			var rec channels.AuditRecord
			var raw string
			if err := r.Scan(&raw); err != nil {
				return rec, err
			}
			if err := json.Unmarshal([]byte(raw), &rec); err != nil {
				return rec, fmt.Errorf("damaged audit record: %w", err)
			}
			rec.At = utc(rec.At)
			return rec, nil
		})
	return out, s.wrap(err, "read audit of %s", id)
}

// Keys implements [channels.Store].
func (s *Store) Keys(ctx context.Context) ([]channels.Key, error) {
	cs, err := s.channels(ctx, "")
	if err != nil {
		return nil, s.wrap(err, "list channels")
	}
	out := make([]channels.Key, len(cs))
	for i, c := range cs {
		out[i] = c.Key
	}
	return out, nil
}

// TypeRecords implements [channels.Store].
func (s *Store) TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error) {
	out, err := s.typeRecords(ctx, s.db)
	return out, s.wrap(err, "read type records")
}

func (s *Store) typeRecords(ctx context.Context, q Querier) (map[string]channels.TypeRecord, error) {
	type row struct{ typ, mode, card string }
	scan := func(r interface{ Scan(...any) error }) (row, error) {
		var p row
		err := r.Scan(&p.typ, &p.mode, &p.card)
		return p, err
	}
	rows, err := collect(ctx, q, `SELECT type, anchor_mode, cardinality FROM channel_types`, nil, scan)
	if err != nil {
		return nil, err
	}
	refs, err := collect(ctx, q, `SELECT type, ref, '' FROM channel_type_refs ORDER BY type, ref`, nil, scan)
	if err != nil {
		return nil, err
	}
	out := make(map[string]channels.TypeRecord, len(rows))
	for _, p := range rows {
		out[p.typ] = channels.TypeRecord{
			Anchor: channels.AnchorMode(p.mode), Cardinality: channels.Cardinality(p.card),
		}
	}
	for _, p := range refs {
		if r, ok := out[p.typ]; ok {
			r.Refs = append(r.Refs, p.mode)
			out[p.typ] = r
		}
	}
	for typ, r := range out {
		slices.Sort(r.Refs)
		out[typ] = r
	}
	return out, nil
}

// typeRecordsLock is the entity lock RecordTypes takes, so two processes
// recording the same type serialize. It cannot collide with an entity id,
// which never holds a slash.
const typeRecordsLock = "channel-types/records"

// typeLock is the entity lock of one channel type, which a channel create
// and DropTypeRecord take. Like typeRecordsLock it holds a slash, so it
// cannot collide with an entity id; and a type name holds none, so it
// cannot collide with typeRecordsLock. It is always the last lock a
// transaction takes, so it adds no lock-order cycle.
func typeLock(typ string) string { return "channel-types/type/" + typ }

// RecordTypes implements [channels.Store]. It reads the held records and
// writes what [channels.MergeTypeRecord] adds in one transaction under
// typeRecordsLock, so a concurrent record cannot slip a ref in between
// the check and the write.
func (s *Store) RecordTypes(ctx context.Context, records map[string]channels.TypeRecord) error {
	records, err := channels.PrepareTypeRecords(records)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	types := slices.Sorted(maps.Keys(records))
	werr := s.write(ctx, []string{typeRecordsLock}, func(q Querier) error {
		cur, err := s.typeRecords(ctx, q)
		if err != nil {
			return err
		}
		for _, typ := range types {
			given := records[typ]
			held, ok := cur[typ]
			added, fill, err := channels.MergeTypeRecord(typ, held, ok, given)
			if err != nil {
				return err
			}
			switch {
			case !ok:
				if _, err := q.Exec(ctx, `INSERT INTO channel_types (type, anchor_mode, cardinality)
					VALUES (?, ?, ?)`, typ, string(given.Anchor), string(given.Cardinality)); err != nil {
					return err
				}
			case fill:
				if _, err := q.Exec(ctx, `UPDATE channel_types SET cardinality = ? WHERE type = ?`,
					string(given.Cardinality), typ); err != nil {
					return err
				}
			}
			for _, ref := range added {
				if _, err := q.Exec(ctx, `INSERT INTO channel_type_refs (type, ref) VALUES (?, ?)`,
					typ, ref); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return s.wrap(werr, "record type records")
}

// Raw renders every row of the channel tables as text, for the conformance
// suite's erasure checks. It is not part of [channels.Store].
func Raw(ctx context.Context, s *Store) ([]byte, error) {
	var b strings.Builder
	for _, q := range []string{
		`SELECT anchor_id || ' ' || anchor_face || ' ' || type || ' ' || name || ' ' || tomb || ' ' || title ||
			' ' || created_by FROM channels`,
		`SELECT doc FROM channel_messages`,
		`SELECT record FROM channel_audit`,
	} {
		lines, err := collect(ctx, s.db, q, nil, func(r interface{ Scan(...any) error }) (string, error) {
			var v string
			err := r.Scan(&v)
			return v, err
		})
		if err != nil {
			return nil, err
		}
		for _, l := range lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return []byte(b.String()), nil
}
