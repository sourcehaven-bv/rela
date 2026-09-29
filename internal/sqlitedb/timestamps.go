package sqlitedb

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TimeFormat is the on-disk format of the store's timestamp columns: RFC 3339
// in UTC with exactly nine fractional digits.
//
// Fixed width is load-bearing. The version sweep compares updated_at and
// created_at as STRINGS in SQL, and string order matches time order only when
// every value has the same width and offset. time.RFC3339Nano trims trailing
// zeros, so "…05.1234Z" sorted after the later "…05.123449999Z" (BUG-HEIAVS).
// The comments table already used this layout for the same reason.
const TimeFormat = "2006-01-02T15:04:05.000000000Z07:00"

// FormatTime renders t in [TimeFormat]. It converts to UTC first, because a
// value with another offset would break string ordering just as surely.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeFormat) }

// timestampColumns are the columns written in [TimeFormat]. Other tables
// (project_files, state_kv) are written by their own packages and never
// compared in SQL.
var timestampColumns = []struct{ table, column string }{
	{"entities", "updated_at"},
	{"relations", "updated_at"},
	{"attachments", "updated_at"},
	{"entity_versions", "created_at"},
	{"relation_versions", "created_at"},
	{"schema_versions", "captured_at"},
}

// normalizeTimestamps rewrites every value in [timestampColumns] to
// [TimeFormat]. Values already in the format are left alone, so a re-run
// changes nothing.
func normalizeTimestamps(ctx context.Context, conn *sql.Conn) error {
	for _, c := range timestampColumns {
		// table and column come from the literals above, never from input.
		if err := normalizeColumn(ctx, conn, c.table, c.column); err != nil {
			return fmt.Errorf("normalize %s.%s: %w", c.table, c.column, err)
		}
	}
	return nil
}

// normalizeBatch is how many rows normalizeColumn reads per page, which bounds
// its memory on a large history table.
const normalizeBatch = 5000

func normalizeColumn(ctx context.Context, conn *sql.Conn, table, column string) error {
	update, err := conn.PrepareContext(ctx, fmt.Sprintf("UPDATE %s SET %s = ? WHERE rowid = ?", table, column))
	if err != nil {
		return err
	}
	defer func() { _ = update.Close() }()
	for after := int64(0); ; {
		changes, last, err := timestampChanges(ctx, conn, table, column, after)
		if err != nil {
			return err
		}
		for _, ch := range changes {
			if _, err := update.ExecContext(ctx, ch.value, ch.rowid); err != nil {
				return err
			}
		}
		if last == after {
			return nil
		}
		after = last
	}
}

// timestampChange is one row whose stored value is not yet in [TimeFormat].
type timestampChange struct {
	rowid int64
	value string
}

// timestampChanges reads one page of rows after rowid `after` and returns
// those to rewrite, plus the last rowid read (equal to `after` when the page
// was empty). The page is collected in full first, so no UPDATE runs while
// this query's rows are still open on the same connection.
func timestampChanges(
	ctx context.Context, conn *sql.Conn, table, column string, after int64,
) (changes []timestampChange, last int64, err error) {
	rows, err := conn.QueryContext(ctx,
		fmt.Sprintf("SELECT rowid, %s FROM %s WHERE rowid > ? ORDER BY rowid LIMIT ?", column, table),
		after, normalizeBatch)
	if err != nil {
		return nil, after, err
	}
	defer func() { _ = rows.Close() }()
	last = after
	for rows.Next() {
		var raw string
		if err := rows.Scan(&last, &raw); err != nil {
			return nil, after, err
		}
		t, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return nil, after, fmt.Errorf("rowid %d: %w", last, err)
		}
		if v := FormatTime(t); v != raw {
			changes = append(changes, timestampChange{last, v})
		}
	}
	return changes, last, rows.Err()
}
