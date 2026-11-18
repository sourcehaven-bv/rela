package sqlaudit

import (
	"context"
	"encoding/json"
	"math"
)

// Newest returns up to limit records with a seq below before, newest
// first. A before of zero or less starts at the newest record. The page is
// cut in SQL on the seq primary key, so its cost does not grow with the
// log.
//
// It is ungated, like [Log.Records]: a request path reads it through
// internal/visibility, which decides who may see the records.
func (l *Log) Newest(ctx context.Context, before int64, limit int) ([]Entry, error) {
	return l.scanEntries(ctx, "newest",
		`SELECT seq, doc FROM audit_log WHERE seq < ? ORDER BY seq DESC LIMIT ?`,
		upperBound(before), limit)
}

// ForEntity returns up to limit records that name entity id, with a seq
// below before, newest first. A record names an id when its subject, its
// before or after subject, or a relation end carries it. The page is read
// through the (entity_id, seq) primary key of audit_subjects.
//
// It is ungated; see [Log.Newest].
func (l *Log) ForEntity(ctx context.Context, id string, before int64, limit int) ([]Entry, error) {
	return l.scanEntries(ctx, "for entity",
		`SELECT l.seq, l.doc FROM audit_subjects s JOIN audit_log l ON l.seq = s.seq
		 WHERE s.entity_id = ? AND s.seq < ? ORDER BY s.seq DESC LIMIT ?`,
		id, upperBound(before), limit)
}

// upperBound turns a page cursor into an exclusive seq bound.
func upperBound(before int64) int64 {
	if before <= 0 {
		return math.MaxInt64
	}
	return before
}

func (l *Log) scanEntries(ctx context.Context, what, query string, args ...any) ([]Entry, error) {
	rows, err := l.db.Query(ctx, query, args...)
	if err != nil {
		return nil, l.errf("%s: %w", what, err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var (
			e   Entry
			doc string
		)
		if err := rows.Scan(&e.Seq, &doc); err != nil {
			return nil, l.errf("%s: %w", what, err)
		}
		if err := json.Unmarshal([]byte(doc), &e.Record); err != nil {
			return nil, l.errf("%s: seq %d: %w", what, e.Seq, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, l.errf("%s: %w", what, err)
	}
	return out, nil
}
