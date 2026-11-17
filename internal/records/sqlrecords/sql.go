// Package sqlrecords is the one implementation of [records.Store] over a
// SQL database. pgrecords and sqliterecords adapt their driver to
// [sqlexec.DB]; the queries live here once, so the two database backends
// cannot drift apart.
//
// # Schema
//
// Two tables, created by pgstore migration 0048 and the sqlitedb v39 rung:
//
//   - records: one row per record. The envelope (type, owner, ts,
//     recorded_at, recorded_by, grp, reverses) is columns; the fields are
//     the JSON doc column. Times are BIGINT microseconds since the epoch.
//     updated_at (0 for never), updated_by and revision carry the last
//     change of a mutable row; pgstore migration 0050 and the sqlitedb v41
//     rung add them.
//   - record_groups: the ledger groups claimed so far, keyed by (type,
//     grp). An append claims its groups with INSERT ... ON CONFLICT DO
//     NOTHING in the same transaction as its rows, so of two appends that
//     claim one group, exactly one commits. A claim lives as long as a row
//     of its group does.
package sqlrecords

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// Store is the SQL backend.
type Store struct {
	db   sqlexec.DB
	name string
}

var (
	_ records.Store   = (*Store)(nil)
	_ records.Mutator = (*Store)(nil)
)

// New returns a store over db. name prefixes error messages.
//
// Nil: db is rejected.
func New(db sqlexec.DB, name string) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("%s: New requires a database handle", name)
	}
	return &Store{db: db, name: name}, nil
}

func (s *Store) wrap(op string, err error) error {
	if err == nil || errors.Is(err, records.ErrGroupExists) || errors.Is(err, records.ErrInvalid) ||
		errors.Is(err, records.ErrNotFound) || errors.Is(err, records.ErrStale) {

		return err
	}
	return fmt.Errorf("%s: %s: %w", s.name, op, err)
}

// Append implements [records.Store.Append].
func (s *Store) Append(ctx context.Context, r records.AppendRequest) error {
	docs := make([]string, len(r.Records))
	for i, rec := range r.Records {
		doc, err := records.EncodeFields(rec.Fields)
		if err != nil {
			return err
		}
		docs[i] = doc
	}
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		for _, g := range r.Groups {
			n, err := q.Exec(ctx,
				`INSERT INTO record_groups (type, grp) VALUES (?, ?) ON CONFLICT DO NOTHING`, g.Type, g.Group)
			if err != nil {
				return err
			}
			if n == 0 {
				return fmt.Errorf("%w: %s", records.ErrGroupExists, g.Group)
			}
		}
		for i, rec := range r.Records {
			if _, err := q.Exec(ctx, `INSERT INTO records
				(id, type, owner, ts, recorded_at, recorded_by, grp, reverses, doc,
				 updated_at, updated_by, revision)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				rec.ID, rec.Type, rec.Owner, records.Micros(rec.TS), records.Micros(rec.RecordedAt),
				rec.RecordedBy, rec.Group, rec.Reverses, docs[i],
				updatedMicros(rec), rec.UpdatedBy, rec.Revision); err != nil {
				return err
			}
		}
		return nil
	})
	return s.wrap("append", err)
}

const cols = `id, type, owner, ts, recorded_at, recorded_by, grp, reverses, doc,
	updated_at, updated_by, revision`

// updatedMicros is rec's updated_at column: 0 for a row never changed.
func updatedMicros(rec records.Record) int64 {
	if rec.UpdatedAt.IsZero() {
		return 0
	}
	return records.Micros(rec.UpdatedAt)
}

func scan(r sqlexec.Rows) (records.Record, error) {
	var rec records.Record
	var ts, at, upd int64
	var doc string
	if err := r.Scan(&rec.ID, &rec.Type, &rec.Owner, &ts, &at, &rec.RecordedBy,
		&rec.Group, &rec.Reverses, &doc, &upd, &rec.UpdatedBy, &rec.Revision); err != nil {
		return rec, err
	}
	rec.TS, rec.RecordedAt = records.FromMicros(ts), records.FromMicros(at)
	if upd != 0 {
		rec.UpdatedAt = records.FromMicros(upd)
	}
	f, err := records.DecodeFields([]byte(doc))
	if err != nil {
		return rec, err
	}
	rec.Fields = f
	return rec, nil
}

// List implements [records.Store.List].
func (s *Store) List(ctx context.Context, q records.ListQuery) (records.Page, error) {
	micros, id, err := records.ParseCursor(q.After)
	if err != nil {
		return records.Page{}, err
	}
	where := []string{"type = ?"}
	args := []any{q.Type}
	if q.Owner != "" {
		where, args = append(where, "owner = ?"), append(args, q.Owner)
	}
	if q.Group != "" {
		where, args = append(where, "grp = ?"), append(args, q.Group)
	}
	if !q.From.IsZero() {
		where, args = append(where, "ts >= ?"), append(args, records.Micros(q.From))
	}
	if !q.To.IsZero() {
		where, args = append(where, "ts < ?"), append(args, records.Micros(q.To))
	}
	if q.After != "" {
		where = append(where, "(ts > ? OR (ts = ? AND id > ?))")
		args = append(args, micros, micros, id)
	}
	limit := records.PageLimit(q)
	args = append(args, limit+1)
	rows, err := s.db.Query(ctx, `SELECT `+cols+` FROM records WHERE `+strings.Join(where, " AND ")+
		` ORDER BY ts, id LIMIT ?`, args...)
	if err != nil {
		return records.Page{}, s.wrap("list", err)
	}
	defer rows.Close()
	var out []records.Record
	for rows.Next() {
		rec, err := scan(rows)
		if err != nil {
			return records.Page{}, s.wrap("list", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return records.Page{}, s.wrap("list", err)
	}
	page := records.Page{Records: out}
	if len(out) > limit {
		page.Records = out[:limit]
		page.Next = records.Cursor(out[limit-1])
	}
	return page, nil
}

// Owners implements [records.Store.Owners].
func (s *Store) Owners(ctx context.Context, typ string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT owner FROM records WHERE type = ? ORDER BY owner`, typ)
	if err != nil {
		return nil, s.wrap("owners", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			return nil, s.wrap("owners", err)
		}
		out = append(out, o)
	}
	return out, s.wrap("owners", rows.Err())
}

// Types implements [records.Store.Types].
func (s *Store) Types(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT type FROM records ORDER BY type`)
	if err != nil {
		return nil, s.wrap("types", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var typ string
		if err := rows.Scan(&typ); err != nil {
			return nil, s.wrap("types", err)
		}
		out = append(out, typ)
	}
	return out, s.wrap("types", rows.Err())
}

// RekeyOwner implements [records.Store.RekeyOwner].
func (s *Store) RekeyOwner(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	_, err := s.db.Exec(ctx, `UPDATE records SET owner = ? WHERE owner = ?`, newID, oldID)
	return s.wrap("rekey", err)
}

// dropOrphanGroups removes the claims no row holds any more.
const dropOrphanGroups = `DELETE FROM record_groups WHERE NOT EXISTS (
	SELECT 1 FROM records r WHERE r.type = record_groups.type AND r.grp = record_groups.grp)`

// DropOwner implements [records.Store.DropOwner].
func (s *Store) DropOwner(ctx context.Context, id string) error {
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		n, err := q.Exec(ctx, `DELETE FROM records WHERE owner = ?`, id)
		if err != nil || n == 0 {
			return err
		}
		_, err = q.Exec(ctx, dropOrphanGroups)
		return err
	})
	return s.wrap("drop", err)
}

// Expire implements [records.Store.Expire].
func (s *Store) Expire(ctx context.Context, typ string, before time.Time) (int64, error) {
	var n int64
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		var err error
		n, err = q.Exec(ctx, `DELETE FROM records WHERE type = ? AND ts < ?`, typ, records.Micros(before))
		if err != nil || n == 0 {
			return err
		}
		_, err = q.Exec(ctx, dropOrphanGroups)
		return err
	})
	return n, s.wrap("expire", err)
}

// Get implements [records.Mutator.Get].
func (s *Store) Get(ctx context.Context, typ, id string) (records.Record, error) {
	rows, err := s.db.Query(ctx, `SELECT `+cols+` FROM records WHERE type = ? AND id = ?`, typ, id)
	if err != nil {
		return records.Record{}, s.wrap("get", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return records.Record{}, s.wrap("get", rerr)
		}
		return records.Record{}, fmt.Errorf("%w: record %s", records.ErrNotFound, id)
	}
	rec, err := scan(rows)
	return rec, s.wrap("get", err)
}

// Replace implements [records.Mutator.Replace]. The revision is the
// condition of the UPDATE itself, so of two replaces that read one
// revision, the second matches no row.
func (s *Store) Replace(ctx context.Context, rec records.Record) error {
	doc, err := records.EncodeFields(rec.Fields)
	if err != nil {
		return err
	}
	err = s.db.Transact(ctx, func(q sqlexec.Querier) error {
		n, uerr := q.Exec(ctx, `UPDATE records SET ts = ?, doc = ?, updated_at = ?, updated_by = ?, revision = ?
			WHERE type = ? AND id = ? AND owner = ? AND revision = ?`,
			records.Micros(rec.TS), doc, updatedMicros(rec), rec.UpdatedBy, rec.Revision,
			rec.Type, rec.ID, rec.Owner, rec.Revision-1)
		if uerr != nil || n == 1 {
			return uerr
		}
		return s.missOrStale(ctx, q, rec.Type, rec.ID, rec.Owner)
	})
	return s.wrap("replace", err)
}

// Delete implements [records.Mutator.Delete].
func (s *Store) Delete(ctx context.Context, typ, id string, revision int64) error {
	err := s.db.Transact(ctx, func(q sqlexec.Querier) error {
		n, err := q.Exec(ctx, `DELETE FROM records WHERE type = ? AND id = ? AND revision = ?`, typ, id, revision)
		if err != nil || n == 1 {
			return err
		}
		return s.missOrStale(ctx, q, typ, id, "")
	})
	return s.wrap("delete", err)
}

// missOrStale tells why a conditional change matched no row: the row is
// gone or has another owner ([records.ErrNotFound]), or its revision moved
// on ([records.ErrStale]). owner empty matches any owner.
func (s *Store) missOrStale(ctx context.Context, q sqlexec.Querier, typ, id, owner string) error {
	rows, err := q.Query(ctx, `SELECT owner FROM records WHERE type = ? AND id = ?`, typ, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return fmt.Errorf("%w: record %s", records.ErrNotFound, id)
	}
	var got string
	if err := rows.Scan(&got); err != nil {
		return err
	}
	if owner != "" && got != owner {
		return fmt.Errorf("%w: record %s", records.ErrNotFound, id)
	}
	return fmt.Errorf("%w: record %s", records.ErrStale, id)
}
