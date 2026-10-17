package sqlchannels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
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

// encodeTypeAfter is the type_after column of f: a JSON object of type to
// microseconds, or "" when f has none.
func encodeTypeAfter(f channels.Filing) string {
	if len(f.TypeAfter) == 0 {
		return ""
	}
	out := make(map[string]int64, len(f.TypeAfter))
	for typ, t := range f.TypeAfter {
		out[typ] = micros(t)
	}
	return mustJSON(out)
}

func decodeTypeAfter(raw string) (map[string]time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	var in map[string]int64
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(in))
	for typ, v := range in {
		out[typ] = fromMicros(v)
	}
	return out, nil
}

// insertFiling records what the batch h filed.
func insertFiling(ctx context.Context, q Querier, h channels.TombstoneHandle, f channels.Filing) error {
	a := h.Anchor()
	_, err := q.Exec(ctx, `INSERT INTO channel_tombstones
		(anchor_id, tomb, anchor_face, policy, purge_after, type_after, due_at, pending)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, h.Stamp(), string(a.Face), string(f.Policy), micros(f.PurgeAfter), encodeTypeAfter(f),
		micros(f.DueAt()), boolInt(f.Pending))
	return err
}

// updateTypeAfter writes the type purge times of the batch h, and the due
// time they give.
func updateTypeAfter(ctx context.Context, q Querier, h channels.TombstoneHandle, f channels.Filing) error {
	_, err := q.Exec(ctx, `UPDATE channel_tombstones SET type_after = ?, due_at = ? WHERE anchor_id = ? AND tomb = ?`,
		encodeTypeAfter(f), micros(f.DueAt()), h.Anchor().ID, h.Stamp())
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
	var face, policy, typeAfter string
	var after int64
	var pending int
	if err := r.Scan(&face, &policy, &after, &typeAfter, &pending); err != nil {
		return out, err
	}
	early, err := decodeTypeAfter(typeAfter)
	if err != nil {
		return out, fmt.Errorf("damaged type_after: %w", err)
	}
	out.face = entity.Face(face)
	out.filing = channels.Filing{
		Policy: channels.TombstonePolicy(policy), PurgeAfter: fromMicros(after), TypeAfter: early,
		Pending: pending != 0,
	}
	return out, nil
}

const filingCols = `anchor_face, policy, purge_after, type_after, pending`

// readFiling returns the filing of the batch h, and whether it has one.
func readFiling(ctx context.Context, q Querier, h channels.TombstoneHandle) (filingRow, bool, error) {
	f, err := scanFiling(q.QueryRow(ctx, `SELECT `+filingCols+` FROM channel_tombstones
		WHERE anchor_id = ? AND tomb = ?`, h.Anchor().ID, h.Stamp()))
	if errors.Is(err, ErrNoRows) {
		return f, false, nil
	}
	return f, err == nil, err
}

// confirmSQL makes the matched filings final; see [channels.Filing.Confirmed].
const confirmSQL = `UPDATE channel_tombstones SET pending = 0,
	policy = CASE WHEN policy = 'orphan' THEN 'detach' ELSE policy END WHERE anchor_id = ?`

// ConfirmTombstone implements [channels.Store].
func (s *Store) ConfirmTombstone(ctx context.Context, h channels.TombstoneHandle) error {
	if err := h.Validate(); err != nil {
		return err
	}
	err := s.write(ctx, []string{h.Anchor().ID}, func(q Querier) error {
		n, err := q.Exec(ctx, confirmSQL+` AND tomb = ?`, h.Anchor().ID, h.Stamp())
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

// ConfirmTombstones implements [channels.Store].
func (s *Store) ConfirmTombstones(ctx context.Context, a channels.Anchor) error {
	if err := a.Validate(); err != nil {
		return err
	}
	err := s.write(ctx, []string{a.ID}, func(q Querier) error {
		var err error
		if a.Face == "" {
			_, err = q.Exec(ctx, confirmSQL, a.ID)
		} else {
			_, err = q.Exec(ctx, confirmSQL+` AND anchor_face = ?`, a.ID, string(a.Face))
		}
		return err
	})
	return s.wrap(err, "confirm tombstones of %s", entity.FormatStateRef(a.ID, a.Face))
}

// Tombstones implements [channels.Store]. A batch filed before
// channel_tombstones existed has no row: it lists with an empty Filing and
// with the anchor of the whole id, since a face batch and a whole-entity
// batch whose channels share one face cannot be told apart from the
// channel rows.
func (s *Store) Tombstones(ctx context.Context, before time.Time) ([]channels.TombstoneBatch, error) {
	bound := channels.StampBefore(before)
	out, err := listBatches(ctx, s.db, `tomb < ?`, []any{bound}, `tomb < ?`, []any{bound})
	return out, s.wrap(err, "list tombstones")
}

// dueFilings selects the filings DueTombstones lists; channel_tombstones_due_idx serves it.
const dueFilings = `pending = 0 AND due_at > 0 AND due_at <= ?`

// DueTombstones implements [channels.Store].
func (s *Store) DueTombstones(ctx context.Context, at time.Time) ([]channels.TombstoneBatch, error) {
	due := micros(channels.NormTime(at))
	out, err := listBatches(ctx, s.db,
		`(anchor_id, tomb) IN (SELECT anchor_id, tomb FROM channel_tombstones WHERE `+dueFilings+`)`, []any{due},
		dueFilings, []any{due})
	return out, s.wrap(err, "list due tombstones")
}

// listBatches groups the tombstoned channel rows matching chanWhere into
// batches, with the filings matching filingWhere.
func listBatches(
	ctx context.Context, q Querier, chanWhere string, chanArgs []any, filingWhere string, filingArgs []any,
) ([]channels.TombstoneBatch, error) {
	type tombRow struct {
		tomb string
		r    row
	}
	rows, err := collect(ctx, q, `SELECT tomb, `+chanCols+` FROM channels
		WHERE tomb <> '' AND `+chanWhere+` ORDER BY tomb, anchor_id, anchor_face, type, name`,
		chanArgs, func(r interface{ Scan(...any) error }) (tombRow, error) {
			var out tombRow
			var tomb string
			wrapped := scanFunc(func(dest ...any) error { return r.Scan(append([]any{&tomb}, dest...)...) })
			cr, err := scanChannel(wrapped)
			out.tomb, out.r = tomb, cr
			return out, err
		})
	if err != nil {
		return nil, err
	}
	filings, err := filingsWhere(ctx, q, filingWhere, filingArgs)
	if err != nil {
		return nil, err
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
			return nil, err
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

// filingsWhere returns the filings matching where, by anchor id and stamp,
// in one query.
func filingsWhere(ctx context.Context, q Querier, where string, args []any) (map[[2]string]filingRow, error) {
	type keyed struct {
		key [2]string
		f   filingRow
	}
	rows, err := collect(ctx, q, `SELECT anchor_id, tomb, `+filingCols+` FROM channel_tombstones WHERE `+where,
		args, func(r interface{ Scan(...any) error }) (keyed, error) {
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

// Purge implements [channels.Store]. A whole purge moves the batch's filed
// trail to the tomb label [channels.PurgedTrail] in the same transaction,
// so a second Purge or an Untombstone no longer finds the batch. Every
// purge writes its records under that label.
func (s *Store) Purge(
	ctx context.Context, h channels.TombstoneHandle, types []string, by channels.Author, at time.Time,
) error {
	if h.IsZero() {
		return nil
	}
	if err := h.Validate(); err != nil {
		return err
	}
	if err := channels.CheckPurge(types, by, at); err != nil {
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
		batch, err := collect(ctx, q, `SELECT `+chanCols+` FROM channels WHERE anchor_id = ? AND tomb = ?
			ORDER BY anchor_face, type, name`, []any{a.ID, h.Stamp()}, scanChannel)
		if err != nil {
			return err
		}
		var filedAudit int
		if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM channel_audit WHERE anchor_id = ? AND tomb = ?`,
			a.ID, h.Stamp()).Scan(&filedAudit); err != nil {
			return err
		}
		if len(batch) == 0 && filedAudit == 0 {
			return fmt.Errorf("%w: tombstone %s", channels.ErrNotFound, h)
		}
		var erased []row
		for _, r := range batch {
			if len(types) == 0 || slices.Contains(types, r.c.Key.Type) {
				erased = append(erased, r)
			}
		}
		whole := len(erased) == len(batch)
		for _, r := range erased {
			// Messages first, for the reason Delete gives.
			if _, err := q.Exec(ctx, `DELETE FROM channel_messages WHERE channel_id = ?`, r.id); err != nil {
				return err
			}
			if _, err := q.Exec(ctx, `DELETE FROM channels WHERE id = ?`, r.id); err != nil {
				return err
			}
		}
		trail := channels.PurgedTrail(h.Stamp())
		if whole {
			if _, err := q.Exec(ctx, `UPDATE channel_audit SET tomb = ? WHERE anchor_id = ? AND tomb = ?`,
				trail, a.ID, h.Stamp()); err != nil {
				return err
			}
		}
		chs := make([]channels.Channel, len(erased))
		for i, r := range erased {
			chs[i] = r.c
		}
		for _, rec := range channels.PurgeAudit(h, chs, by, at) {
			if _, err := q.Exec(ctx, `INSERT INTO channel_audit (anchor_id, tomb, record) VALUES (?, ?, ?)`,
				a.ID, trail, mustJSON(rec)); err != nil {
				return err
			}
		}
		if whole {
			return deleteFiling(ctx, q, h)
		}
		if !ok || len(f.filing.TypeAfter) == 0 {
			return nil
		}
		rest := maps.Clone(f.filing.TypeAfter)
		for _, typ := range types {
			delete(rest, typ)
		}
		f.filing.TypeAfter = rest
		return updateTypeAfter(ctx, q, h, f.filing)
	})
	return s.wrap(err, "purge %s", entity.FormatStateRef(a.ID, a.Face))
}

// DropTypeRecord implements [channels.Store]. It runs under the lock
// RecordTypes takes and under the type's own lock, which a channel create
// takes too (see typeLock), so a drop serializes with a record and with a
// create of the type on every process. channels_type_state_idx serves the
// count.
func (s *Store) DropTypeRecord(ctx context.Context, typ string) error {
	if err := channels.CheckDropType(typ); err != nil {
		return err
	}
	err := s.write(ctx, []string{typeRecordsLock, typeLock(typ)}, func(q Querier) error {
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
