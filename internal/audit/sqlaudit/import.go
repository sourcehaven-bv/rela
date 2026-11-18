package sqlaudit

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// errNilLog is a [NewSink] call without a log.
var errNilLog = errors.New("sqlaudit: a log is required")

// Import appends every record recs yields, in order, and then an
// [audit.OpAuditImport] record by rec's principal whose summary is
// summary, all in one transaction: an import that fails leaves nothing
// behind and can be run again. It returns how many records it appended.
//
// An imported row's append time is the record's own time (now when the
// record has none), so retention counts from when the record was written.
// A record of a log-maintenance operation ([audit.OpAuditPrune],
// [audit.OpAuditImport]) is refused: those describe another database's
// chain, and a planted prune record would vouch for a forged anchor.
func (l *Log) Import(
	ctx context.Context, recs iter.Seq2[audit.Record, error], rec audit.Record, summary string,
) (int64, error) {
	var n int64
	err := l.db.Transact(ctx, func(q sqlexec.Querier) error {
		c, err := l.begin(ctx, q)
		if err != nil {
			return err
		}
		for r, err := range recs {
			if err != nil {
				return err
			}
			if r.Op == audit.OpAuditPrune || r.Op == audit.OpAuditImport {
				return l.errf("import: record %d is a %s record, which only this log may write", n+1, r.Op)
			}
			at := l.now()
			if !r.Time.IsZero() {
				at = r.Time.UTC().UnixMicro()
			}
			if err := c.append(ctx, r, at); err != nil {
				return err
			}
			n++
		}
		rec.Op = audit.OpAuditImport
		rec.Subject, rec.Before, rec.After = nil, nil, nil
		rec.Summary = fmt.Sprintf("imported records=%d %s", n, summary)
		return c.append(ctx, rec, l.now())
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}
