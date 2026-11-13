// Package sqlrecords is the one implementation of [records.Store] over a
// SQL database. pgrecords and sqliterecords adapt their driver to
// [sqlexec.DB]; the queries live here once, so the two database backends
// cannot drift apart.
//
// # Schema
//
// Two tables, created by pgstore migration 0045 and the sqlitedb v37 rung:
//
//   - records: one row per record. The envelope (type, owner, ts,
//     recorded_at, recorded_by, grp, reverses) is columns; the fields are
//     the JSON doc column. Times are BIGINT microseconds since the epoch.
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

var _ records.Store = (*Store)(nil)

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
	if err == nil || errors.Is(err, records.ErrGroupExists) || errors.Is(err, records.ErrInvalid) {
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
				(id, type, owner, ts, recorded_at, recorded_by, grp, reverses, doc)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				rec.ID, rec.Type, rec.Owner, records.Micros(rec.TS), records.Micros(rec.RecordedAt),
				rec.RecordedBy, rec.Group, rec.Reverses, docs[i]); err != nil {
				return err
			}
		}
		return nil
	})
	return s.wrap("append", err)
}

const cols = `id, type, owner, ts, recorded_at, recorded_by, grp, reverses, doc`

func scan(r sqlexec.Rows) (records.Record, error) {
	var rec records.Record
	var ts, at int64
	var doc string
	if err := r.Scan(&rec.ID, &rec.Type, &rec.Owner, &ts, &at, &rec.RecordedBy,
		&rec.Group, &rec.Reverses, &doc); err != nil {
		return rec, err
	}
	rec.TS, rec.RecordedAt = records.FromMicros(ts), records.FromMicros(at)
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
