package sqlaudit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// Pruned is the result of [Log.Prune].
type Pruned struct {
	// Rows is how many rows were deleted.
	Rows int64
	// Anchor is the seq of the last deleted row, which the chain now starts
	// after. 0 when nothing was deleted.
	Anchor int64
}

// ErrChainBroken is a [Log.Prune] refused because a row it would delete
// does not verify: deleting it would erase the evidence.
var ErrChainBroken = errors.New("audit chain broken")

// Prune deletes the oldest rows appended before cutoff and appends an
// [audit.OpAuditPrune] record, by rec's principal, that names the new
// anchor: the seq and hash of the last deleted row. [Log.Verify] then
// still checks every remaining row and the anchor. Nothing is deleted, and
// nothing recorded, when no row qualifies.
//
// Rows are deleted by seq, from the anchor up to the row before the first
// one appended at or after cutoff, so the rows that remain stay gapless and
// a row with a wrong clock cannot pull newer rows in. The rows to delete are
// verified first, in the same transaction; a break refuses the prune with
// [ErrChainBroken].
//
// The caller enforces the retention minimum; this deletes what it is told.
func (l *Log) Prune(ctx context.Context, cutoff time.Time, rec audit.Record) (Pruned, error) {
	var out Pruned
	cut := cutoff.UTC().UnixMicro()
	err := l.db.Transact(ctx, func(q sqlexec.Querier) error {
		c, err := l.begin(ctx, q)
		if err != nil {
			return err
		}
		anchorSeq, anchorHash, err := l.anchor(ctx, q)
		if err != nil {
			return err
		}
		last, err := l.pruneEnd(ctx, q, cut, anchorSeq, c.seq)
		if err != nil || last == 0 {
			return err
		}
		v := Verification{Anchor: anchorSeq, HeadSeq: anchorSeq, HeadHash: anchorHash}
		if walkErr := l.walk(ctx, q, &v, last); walkErr != nil {
			return walkErr
		}
		if v.Break == nil && v.HeadSeq != last { // coverage-ignore: defensive: pruneEnd read last from the table
			v.Break = &Break{Seq: v.HeadSeq + 1, Reason: "rows are missing"}
		}
		if v.Break != nil {
			return fmt.Errorf("%w at seq %d: %s; prune refused", ErrChainBroken, v.Break.Seq, v.Break.Reason)
		}
		_, err = q.Exec(ctx, `INSERT INTO audit_anchor (id, seq, hash, pruned_at) VALUES (1, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET seq = excluded.seq, hash = excluded.hash, pruned_at = excluded.pruned_at`,
			last, v.HeadHash, l.now())
		if err != nil {
			return l.errf("prune: set anchor: %w", err)
		}
		if _, err = q.Exec(ctx, `DELETE FROM audit_subjects WHERE seq <= ?`, last); err != nil {
			return l.errf("prune: %w", err)
		}
		n, err := q.Exec(ctx, `DELETE FROM audit_log WHERE seq <= ?`, last)
		if err != nil {
			return l.errf("prune: %w", err)
		}
		out = Pruned{Rows: n, Anchor: last}
		rec.Op = audit.OpAuditPrune
		rec.Subject, rec.Before, rec.After = nil, nil, nil
		rec.Summary = fmt.Sprintf(pruneSummary, last, v.HeadHash, n, cutoff.UTC().Format(time.RFC3339))
		return c.append(ctx, rec, l.now())
	})
	if err != nil {
		return Pruned{}, err
	}
	return out, nil
}

// pruneEnd returns the seq of the last row to delete, or 0 when none: the
// row before the first one appended at or after cut, or the head when every
// row is older.
func (l *Log) pruneEnd(ctx context.Context, q sqlexec.Querier, cut, anchorSeq, headSeq int64) (int64, error) {
	var young int64
	if err := q.QueryRow(ctx, `SELECT COALESCE(MIN(seq), 0) FROM audit_log WHERE at >= ?`, cut).
		Scan(&young); err != nil {
		return 0, l.errf("prune: %w", err)
	}
	last := headSeq
	if young > 0 {
		last = young - 1
	}
	if last <= anchorSeq {
		return 0, nil
	}
	return last, nil
}
