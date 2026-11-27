package sqlrecords

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/records"
)

var _ records.Aggregator = (*Store)(nil)

// ownerChunk bounds the owners bound into one statement, under the
// parameter limit of both databases.
const ownerChunk = 500

const (
	microsPerHour = int64(time.Hour / time.Microsecond)
	microsPerDay  = 24 * microsPerHour
	microsPerWeek = 7 * microsPerDay
	// weekOffset moves the epoch (a Thursday) onto a Monday: a week starts
	// at ts - ((ts + weekOffset) mod microsPerWeek).
	weekOffset = 3 * microsPerDay
)

// Aggregate implements [records.Aggregator]. It has no row bound: one
// statement per chunk of owners folds in the database or streams only the
// columns the fold reads.
//
// A count over hour, day, week or no bucket is a GROUP BY. A fold of a
// field, or a month bucket, streams rows into [records.Folder] instead:
// exact decimal sums have no portable SQL form, since SQLite has no decimal
// type, and a month start is calendar arithmetic that differs per
// database.
func (s *Store) Aggregate(ctx context.Context, q records.AggregateQuery, scale int) ([]records.Point, error) {
	grouped := q.Field == "" && q.Bucket != records.BucketMonth
	var folder *records.Folder
	var merged map[string]*records.Point
	if grouped {
		merged = map[string]*records.Point{}
	} else {
		folder = records.NewFolder(q, scale)
	}
	for start := 0; start < len(q.Owners); start += ownerChunk {
		chunk := q.Owners[start:min(start+ownerChunk, len(q.Owners))]
		where, args := aggregateWhere(q, chunk)
		var err error
		if grouped {
			err = s.groupCount(ctx, q, where, args, merged)
		} else {
			err = s.streamFold(ctx, q, where, args, folder)
		}
		if err != nil {
			return nil, s.wrap("aggregate", err)
		}
	}
	if !grouped {
		return folder.Points(), nil
	}
	out := make([]records.Point, 0, len(merged))
	for _, p := range merged {
		out = append(out, *p)
	}
	slices.SortFunc(out, func(a, b records.Point) int {
		return cmp.Or(
			cmp.Compare(a.Owner, b.Owner),
			a.Start.Compare(b.Start),
		)
	})
	return out, nil
}

func aggregateWhere(q records.AggregateQuery, owners []string) (clause string, args []any) {
	where := []string{"type = ?", "owner IN (" + strings.TrimSuffix(strings.Repeat("?, ", len(owners)), ", ") + ")"}
	args = []any{q.Type}
	for _, o := range owners {
		args = append(args, o)
	}
	if !q.From.IsZero() {
		where, args = append(where, "ts >= ?"), append(args, records.Micros(q.From))
	}
	if !q.To.IsZero() {
		where, args = append(where, "ts < ?"), append(args, records.Micros(q.To))
	}
	return strings.Join(where, " AND "), args
}

// bucketExpr is the SQL for the start of the bucket ts falls in. Both
// databases truncate a remainder toward zero, so the double modulo keeps a
// time before the epoch in its own bucket.
func bucketExpr(b records.Bucket) string {
	floor := func(width, offset int64) string {
		return fmt.Sprintf("ts - (((ts + %d) %% %d) + %d) %% %d", offset, width, width, width)
	}
	switch b {
	case records.BucketHour:
		return floor(microsPerHour, 0)
	case records.BucketDay:
		return floor(microsPerDay, 0)
	case records.BucketWeek:
		return floor(microsPerWeek, weekOffset)
	case records.BucketNone, records.BucketMonth:
	}
	return "0"
}

func (s *Store) groupCount(ctx context.Context, q records.AggregateQuery, where string, args []any,
	merged map[string]*records.Point) error {
	owner := "''"
	if q.PerOwner {
		owner = "owner"
	}
	rows, err := s.db.Query(ctx, `SELECT `+owner+`, `+bucketExpr(q.Bucket)+`, COUNT(*) FROM records WHERE `+
		where+` GROUP BY 1, 2`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var o string
		var start, n int64
		if err := rows.Scan(&o, &start, &n); err != nil {
			return err
		}
		pt := records.Point{Owner: o, Count: n}
		if q.Bucket != records.BucketNone {
			pt.Start = records.FromMicros(start)
		}
		key := o + "\x00" + pt.Start.Format(time.RFC3339)
		if have := merged[key]; have != nil {
			have.Count += n
			continue
		}
		merged[key] = &pt
	}
	return rows.Err()
}

func (s *Store) streamFold(ctx context.Context, q records.AggregateQuery, where string, args []any,
	folder *records.Folder) error {
	col := "''"
	if q.Field != "" {
		col = "doc"
	}
	rows, err := s.db.Query(ctx, `SELECT id, owner, ts, `+col+` FROM records WHERE `+where, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rec records.Record
		var ts int64
		var doc string
		if scanErr := rows.Scan(&rec.ID, &rec.Owner, &ts, &doc); scanErr != nil {
			return scanErr
		}
		rec.TS = records.FromMicros(ts)
		if q.Field != "" {
			if rec.Fields, err = records.DecodeFields([]byte(doc)); err != nil {
				return err
			}
		}
		folder.Add(rec)
	}
	return rows.Err()
}
