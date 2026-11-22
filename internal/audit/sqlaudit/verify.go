package sqlaudit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// verifyPage is how many rows [Log.Verify] reads per query, so a long log is
// walked in bounded memory.
const verifyPage = 1000

// verifyRestarts bounds how often [Log.Verify] starts over because a prune
// moved the anchor while it walked.
const verifyRestarts = 3

// Verification is the result of [Log.Verify].
type Verification struct {
	// Rows is how many rows were checked.
	Rows int64
	// Anchor is the seq of the last pruned row, 0 when nothing was pruned.
	// The first row must link to it.
	Anchor int64
	// HeadSeq and HeadHash identify the last row checked. Keep them outside
	// the database: a later Verify whose chain does not pass through them
	// shows that the log was rewritten.
	HeadSeq  int64
	HeadHash string
	// Break is the first row that does not verify, nil when every row does.
	Break *Break
}

// Break describes the first row at which the chain does not verify.
type Break struct {
	// Seq is the row's sequence number, or the first missing one.
	Seq    int64
	Reason string
}

// Verify walks the log in seq order and checks that the rows are gapless,
// that each links to its predecessor, that each hash matches its row, and
// that the newest [audit.OpAuditPrune] row names the anchor. It stops at
// the first break. Rows appended while it runs may or may not be included;
// every row it reads is committed. A prune that commits while it walks
// makes it start over.
func (l *Log) Verify(ctx context.Context) (Verification, error) {
	for attempt := 0; ; attempt++ {
		v, err := l.verifyOnce(ctx)
		if err != nil || v.Break == nil || attempt == verifyRestarts {
			return v, err
		}
		moved, err := l.anchorMoved(ctx, v.Anchor)
		if err != nil {
			return Verification{}, err
		}
		if !moved {
			return v, nil
		}
	}
}

func (l *Log) verifyOnce(ctx context.Context) (Verification, error) {
	anchorSeq, anchorHash, err := l.anchor(ctx, l.db)
	if err != nil {
		return Verification{}, err
	}
	v := Verification{Anchor: anchorSeq, HeadSeq: anchorSeq, HeadHash: anchorHash}
	if walkErr := l.walk(ctx, l.db, &v, 0); walkErr != nil {
		return Verification{}, walkErr
	}
	if v.Break == nil && anchorSeq > 0 {
		reason, err := l.checkAnchor(ctx, l.db, anchorSeq, anchorHash)
		if err != nil {
			return Verification{}, err
		}
		if reason != "" {
			v.Break = &Break{Seq: anchorSeq, Reason: reason}
		}
	}
	return v, nil
}

// anchorMoved reports whether the anchor is no longer at seq.
func (l *Log) anchorMoved(ctx context.Context, seq int64) (bool, error) {
	now, _, err := l.anchor(ctx, l.db)
	return now != seq, err
}

// walk checks the rows after v.HeadSeq, through upTo (every row when upTo
// is 0), page by page, and advances v until a break.
func (l *Log) walk(ctx context.Context, q sqlexec.Querier, v *Verification, upTo int64) error {
	for {
		n, err := l.walkPage(ctx, q, v, upTo)
		if err != nil {
			return err
		}
		if n < verifyPage || v.Break != nil {
			return nil
		}
	}
}

// walkPage checks up to [verifyPage] rows after v.HeadSeq and returns how
// many it read.
func (l *Log) walkPage(ctx context.Context, q sqlexec.Querier, v *Verification, upTo int64) (int, error) {
	limit := int64(1<<62 - 1)
	if upTo > 0 {
		limit = upTo
	}
	rows, err := q.Query(ctx, `SELECT seq, at, op, principal_user, doc, prev_hash, hash FROM audit_log
		WHERE seq > ? AND seq <= ? ORDER BY seq LIMIT ?`, v.HeadSeq, limit, verifyPage)
	if err != nil {
		return 0, l.errf("verify: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var (
			r               Row
			doc, prev, hash string
		)
		if err := rows.Scan(&r.Seq, &r.At, &r.Op, &r.User, &doc, &prev, &hash); err != nil {
			return 0, l.errf("verify: %w", err)
		}
		r.Doc = []byte(doc)
		n++
		if b := checkRow(v.HeadSeq, v.HeadHash, r, prev, hash); b != nil {
			v.Break = b
			return n, nil
		}
		v.Rows++
		v.HeadSeq, v.HeadHash = r.Seq, hash
	}
	if err := rows.Err(); err != nil {
		return 0, l.errf("verify: %w", err)
	}
	return n, nil
}

// checkRow checks one row against the head before it.
func checkRow(headSeq int64, headHash string, r Row, prev, hash string) *Break {
	switch {
	case r.Seq != headSeq+1:
		return &Break{Seq: headSeq + 1, Reason: fmt.Sprintf("rows %d to %d are missing", headSeq+1, r.Seq-1)}
	case prev != headHash:
		return &Break{Seq: r.Seq, Reason: "the row does not link to the hash of the row before it"}
	case hash != ChainHash(prev, r):
		return &Break{Seq: r.Seq, Reason: "the row's content does not match its hash"}
	}
	return nil
}

// pruneSummary is the summary of an [audit.OpAuditPrune] record. Its first
// fields are what checkAnchor reads back.
const pruneSummary = "pruned through seq=%d hash=%s rows=%d appended before %s"

// checkAnchor checks that the newest prune row names the anchor, and
// returns why not, or "" when it does. Only a prune sets the anchor, and it
// records the anchor in the same transaction, in a row the chain covers.
func (l *Log) checkAnchor(ctx context.Context, q sqlexec.Querier, seq int64, hash string) (string, error) {
	var doc string
	err := q.QueryRow(ctx, `SELECT doc FROM audit_log WHERE op = ? ORDER BY seq DESC LIMIT 1`,
		audit.OpAuditPrune).Scan(&doc)
	if errors.Is(err, sqlexec.ErrNoRows) {
		return "no audit-prune row records the anchor", nil
	}
	if err != nil {
		return "", l.errf("verify anchor: %w", err)
	}
	var (
		rec     audit.Record
		gotSeq  int64
		gotHash string
	)
	if json.Unmarshal([]byte(doc), &rec) != nil {
		//nolint:nilerr // an undecodable prune row is a verdict on the chain, not a failure to check it
		return "the newest audit-prune row does not decode", nil
	}
	_, scanErr := fmt.Sscanf(rec.Summary, "pruned through seq=%d hash=%s", &gotSeq, &gotHash)
	if scanErr != nil || gotSeq != seq || gotHash != hash {
		//nolint:nilerr // a summary that does not parse is a verdict on the chain, not a failure to check it
		return "the anchor is not the one the newest audit-prune row set", nil
	}
	return "", nil
}

// HashAt returns the stored hash of the row at seq, and false when the log
// keeps no row at seq (never appended, or pruned). The hash is only
// trustworthy after a [Log.Verify] that found no break.
func (l *Log) HashAt(ctx context.Context, seq int64) (hash string, found bool, err error) {
	err = l.db.QueryRow(ctx, `SELECT hash FROM audit_log WHERE seq = ?`, seq).Scan(&hash)
	if errors.Is(err, sqlexec.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, l.errf("hash at %d: %w", seq, err)
	}
	return hash, true, nil
}

// Entry is [audit.Entry], kept so callers of this package need not import
// both.
type Entry = audit.Entry

// Records returns up to limit records appended after seq after, in seq
// order. It is ungated: callers are operator tools and tests, not request
// paths.
func (l *Log) Records(ctx context.Context, after int64, limit int) ([]Entry, error) {
	return l.scanEntries(ctx, "records",
		`SELECT seq, doc FROM audit_log WHERE seq > ? ORDER BY seq LIMIT ?`, after, limit)
}
