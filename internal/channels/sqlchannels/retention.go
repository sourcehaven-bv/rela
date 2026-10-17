package sqlchannels

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The retention half of the store: what a tombstoned batch recorded, and
// the methods that list, confirm and purge batches. The helpers are free
// functions so the Store's method count stays the interface's.

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// insertFiling records what the batch h filed.
func insertFiling(ctx context.Context, q Querier, h channels.TombstoneHandle, f channels.Filing) error {
	a := h.Anchor()
	_, err := q.Exec(ctx, `INSERT INTO channel_tombstones (anchor_id, tomb, anchor_face, policy, purge_after, pending)
		VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, h.Stamp(), string(a.Face), string(f.Policy), micros(f.PurgeAfter), boolInt(f.Pending))
	return err
}

// deleteFiling drops the filing of the batch h, which no longer exists.
func deleteFiling(ctx context.Context, q Querier, h channels.TombstoneHandle) error {
	_, err := q.Exec(ctx, `DELETE FROM channel_tombstones WHERE anchor_id = ? AND tomb = ?`,
		h.Anchor().ID, h.Stamp())
	return err
}

// filingRow is one row of channel_tombstones.
type filingRow struct {
	face   entity.Face
	filing channels.Filing
}

func scanFiling(r interface{ Scan(...any) error }) (filingRow, error) {
	var out filingRow
	var face, policy string
	var after int64
	var pending int
	if err := r.Scan(&face, &policy, &after, &pending); err != nil {
		return out, err
	}
	out.face = entity.Face(face)
	out.filing = channels.Filing{
		Policy: channels.TombstonePolicy(policy), PurgeAfter: fromMicros(after), Pending: pending != 0,
	}
	return out, nil
}

const filingCols = `anchor_face, policy, purge_after, pending`

// readFiling returns the filing of the batch h, and whether it has one.
func readFiling(ctx context.Context, q Querier, h channels.TombstoneHandle) (filingRow, bool, error) {
	f, err := scanFiling(q.QueryRow(ctx, `SELECT `+filingCols+` FROM channel_tombstones
		WHERE anchor_id = ? AND tomb = ?`, h.Anchor().ID, h.Stamp()))
	if errors.Is(err, ErrNoRows) {
		return f, false, nil
	}
	return f, err == nil, err
}

// ConfirmTombstone implements [channels.Store].
func (s *Store) ConfirmTombstone(ctx context.Context, h channels.TombstoneHandle) error {
	if err := h.Validate(); err != nil {
		return err
	}
	err := s.write(ctx, []string{h.Anchor().ID}, func(q Querier) error {
		n, err := q.Exec(ctx, `UPDATE channel_tombstones SET pending = 0 WHERE anchor_id = ? AND tomb = ?`,
			h.Anchor().ID, h.Stamp())
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
		}
		return nil
	})
	return s.wrap(err, "confirm tombstone %s", h)
}

// Tombstones implements [channels.Store]. A batch filed before
// channel_tombstones existed has no row: it lists with an empty Filing and
// with the anchor of the whole id, since a face batch and a whole-entity
// batch whose channels share one face cannot be told apart from the
// channel rows.
func (s *Store) Tombstones(ctx context.Context, before time.Time) ([]channels.TombstoneBatch, error) {
	type tombRow struct {
		tomb string
		r    row
	}
	rows, err := collect(ctx, s.db, `SELECT tomb, `+chanCols+` FROM channels
		WHERE tomb <> '' AND tomb < ? ORDER BY tomb, anchor_id, anchor_face, type, name`,
		[]any{channels.StampBefore(before)}, func(r interface{ Scan(...any) error }) (tombRow, error) {
			var out tombRow
			var tomb string
			wrapped := scanFunc(func(dest ...any) error { return r.Scan(append([]any{&tomb}, dest...)...) })
			cr, err := scanChannel(wrapped)
			out.tomb, out.r = tomb, cr
			return out, err
		})
	if err != nil {
		return nil, s.wrap(err, "list tombstones")
	}
	filings, err := filingsBefore(ctx, s.db, before)
	if err != nil {
		return nil, s.wrap(err, "list tombstones")
	}
	out := []channels.TombstoneBatch{}
	for _, tr := range rows {
		id := tr.r.c.Key.ID
		if n := len(out); n > 0 && out[n-1].Handle.Stamp() == tr.tomb && out[n-1].Handle.Anchor().ID == id {
			out[n-1].Channels = append(out[n-1].Channels, tr.r.c)
			continue
		}
		at, err := channels.StampTime(tr.tomb)
		if err != nil {
			return nil, s.wrap(err, "list tombstones")
		}
		f := filings[[2]string{id, tr.tomb}]
		out = append(out, channels.TombstoneBatch{
			Handle:   channels.NewTombstoneHandle(channels.Anchor{ID: id, Face: f.face}, tr.tomb),
			At:       at,
			Filing:   f.filing,
			Channels: []channels.Channel{tr.r.c},
		})
	}
	return out, nil
}

// filingsBefore returns the filing of every batch filed before the given time,
// by anchor id and stamp, in one query.
func filingsBefore(ctx context.Context, q Querier, before time.Time) (map[[2]string]filingRow, error) {
	type keyed struct {
		key [2]string
		f   filingRow
	}
	rows, err := collect(ctx, q, `SELECT anchor_id, tomb, `+filingCols+` FROM channel_tombstones WHERE tomb < ?`,
		[]any{channels.StampBefore(before)}, func(r interface{ Scan(...any) error }) (keyed, error) {
			var out keyed
			var id, tomb string
			f, err := scanFiling(scanFunc(func(dest ...any) error {
				return r.Scan(append([]any{&id, &tomb}, dest...)...)
			}))
			out.key, out.f = [2]string{id, tomb}, f
			return out, err
		})
	if err != nil {
		return nil, err
	}
	out := make(map[[2]string]filingRow, len(rows))
	for _, r := range rows {
		out[r.key] = r.f
	}
	return out, nil
}

// scanFunc adapts a function to the Scan interface scanChannel takes.
type scanFunc func(dest ...any) error

func (f scanFunc) Scan(dest ...any) error { return f(dest...) }

// Purge implements [channels.Store]. The batch's filed trail moves to the
// tomb label [channels.PurgedTrail] in the same transaction, so a second
// Purge or an Untombstone no longer finds the batch.
func (s *Store) Purge(ctx context.Context, h channels.TombstoneHandle, by channels.Author, at time.Time) error {
	if h.IsZero() {
		return nil
	}
	if err := h.Validate(); err != nil {
		return err
	}
	if err := channels.CheckPurge(by, at); err != nil {
		return err
	}
	a := h.Anchor()
	err := s.write(ctx, []string{a.ID}, func(q Querier) error {
		f, ok, err := readFiling(ctx, q, h)
		if err != nil {
			return err
		}
		if ok && f.filing.Pending {
			return fmt.Errorf("%w: tombstone %s waits for its delete to commit", channels.ErrConflict, h)
		}
		var filedChannels, filedAudit int
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
		// Messages first, for the reason Delete gives.
		if _, err := q.Exec(ctx, `DELETE FROM channel_messages WHERE channel_id IN
			(SELECT id FROM channels WHERE anchor_id = ? AND tomb = ?)`, a.ID, h.Stamp()); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM channels WHERE anchor_id = ? AND tomb = ?`,
			a.ID, h.Stamp()); err != nil {
			return err
		}
		trail := channels.PurgedTrail(h.Stamp())
		if _, err := q.Exec(ctx, `UPDATE channel_audit SET tomb = ? WHERE anchor_id = ? AND tomb = ?`,
			trail, a.ID, h.Stamp()); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `INSERT INTO channel_audit (anchor_id, tomb, record) VALUES (?, ?, ?)`,
			a.ID, trail, mustJSON(channels.PurgeAudit(h, by, at))); err != nil {
			return err
		}
		return deleteFiling(ctx, q, h)
	})
	return s.wrap(err, "purge %s", entity.FormatStateRef(a.ID, a.Face))
}

// DropTypeRecord implements [channels.Store]. It runs under the lock
// RecordTypes takes, so a drop and a record of one type serialize.
func (s *Store) DropTypeRecord(ctx context.Context, typ string) error {
	if err := channels.CheckDropType(typ); err != nil {
		return err
	}
	err := s.write(ctx, []string{typeRecordsLock}, func(q Querier) error {
		var n int
		if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM channels WHERE type = ?`, typ).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%w: %s has %d channels", channels.ErrTypeInUse, typ, n)
		}
		if _, err := q.Exec(ctx, `DELETE FROM channel_type_refs WHERE type = ?`, typ); err != nil {
			return err
		}
		_, err := q.Exec(ctx, `DELETE FROM channel_types WHERE type = ?`, typ)
		return err
	})
	return s.wrap(err, "drop type record %s", typ)
}
