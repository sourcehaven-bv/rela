package sqlaudit

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/audit"
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

// SubjectQuery is [audit.SubjectQuery].
type SubjectQuery = audit.SubjectQuery

// ForEntity returns up to limit records that name q.ID at q.Face, with an op
// in q.Ops and a seq below before, newest first. A record names an id when
// its subject, its before or after subject, or a relation end carries it.
//
// The records start at the latest one that began a lifecycle of that id and
// face: its create, or a rename or conversion into it. So an id that was
// deleted and created again, or renamed away and reused, does not show the
// earlier entity's records. Only a record that names the id at q.Face
// itself marks a start: a row stored with an empty face (one written before
// faces were recorded, or naming the id at two faces) is shown on every face
// but begins no face's lifecycle, so an old record cannot hide another
// face's history.
//
// The op filter and the page cut run in SQL, so a flood of other ops cannot
// empty a page. A page reads the index range of this id, so its cost grows
// with the records of this id, not with the log.
//
// It is ungated; see [Log.Newest].
func (l *Log) ForEntity(ctx context.Context, q SubjectQuery, before int64, limit int) ([]Entry, error) {
	if len(q.Ops) == 0 {
		return nil, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(q.Ops)), ", ")
	args := make([]any, 0, len(q.Ops)+6)
	args = append(args, q.ID, q.Face, upperBound(before))
	for _, op := range q.Ops {
		args = append(args, op)
	}
	args = append(args, q.ID, q.Face, limit)
	return l.scanEntries(ctx, "for entity",
		`SELECT l.seq, l.doc FROM audit_subjects s JOIN audit_log l ON l.seq = s.seq
		 WHERE s.entity_id = ? AND s.face IN (?, '') AND s.seq < ? AND l.op IN (`+marks+`)
		   AND s.seq >= COALESCE((SELECT MAX(f.seq) FROM audit_subjects f
		       WHERE f.entity_id = ? AND f.face = ? AND f.starts), 0)
		 ORDER BY s.seq DESC LIMIT ?`,
		args...)
}

// LifecycleStart returns the record that began the latest lifecycle of q.ID
// at q.Face, the one [Log.ForEntity] reads from, and false when there is
// none (the id never started at that face, or the record was pruned). The
// caller checks that it started the entity it asked about.
//
// It is ungated; see [Log.Newest].
func (l *Log) LifecycleStart(ctx context.Context, q SubjectQuery) (start Entry, found bool, err error) {
	entries, err := l.scanEntries(ctx, "lifecycle start",
		`SELECT l.seq, l.doc FROM audit_subjects f JOIN audit_log l ON l.seq = f.seq
		 WHERE f.entity_id = ? AND f.face = ? AND f.starts
		 ORDER BY f.seq DESC LIMIT 1`,
		q.ID, q.Face)
	if err != nil || len(entries) == 0 {
		return Entry{}, false, err
	}
	return entries[0], true, nil
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
