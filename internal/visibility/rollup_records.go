package visibility

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RecordAggregator folds record rows under the caller's access (TKT-VMSWSWP):
// the owners the caller may not read are dropped before anything folds. It
// answers with the value of each owner that has one, as exact decimal text.
// The wiring site adapts the record service to it.
type RecordAggregator interface {
	FoldRecords(ctx context.Context, q rollup.RecordQuery) (map[string]string, error)
}

// WithRecords returns a copy of r that also computes roll-ups over record
// types through recs, with now as the clock of a window. A Rollups without
// it leaves such a roll-up out, with a log line.
//
// Nil: recs accepted, and then roll-ups over records are left out. now
// accepted; nil means time.Now.
func (r *Rollups) WithRecords(recs RecordAggregator, now func() time.Time) *Rollups {
	if r == nil {
		return nil
	}
	c := *r
	c.recs, c.now = recs, now
	if c.now == nil {
		c.now = time.Now
	}
	return &c
}

// recordColumn runs one roll-up over records for ids: one fold for the
// whole page. The aggregator gates the owners before it folds, so a hidden
// owner's rows never reach a value.
func (r *Rollups) recordColumn(
	ctx context.Context, col rollup.Column, ids []string,
) (map[string]store.RollupResult, error) {
	if r.recs == nil {
		return nil, errors.New("no record service is wired")
	}
	texts, err := r.recs.FoldRecords(ctx, col.RecordQuery(ids, r.now()))
	if err != nil {
		return nil, fmt.Errorf("fold %s: %w", col.Records, err)
	}
	out := make(map[string]store.RollupResult, len(texts))
	for owner, text := range texts {
		if col.Decimal {
			out[owner] = store.RollupResult{Value: text}
			continue
		}
		n, perr := strconv.ParseInt(text, 10, 64)
		if perr != nil {
			continue // outside int64: no value, as a relation sum
		}
		out[owner] = store.RollupResult{Value: n}
	}
	return out, nil
}
