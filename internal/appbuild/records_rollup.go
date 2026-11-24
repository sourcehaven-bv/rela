package appbuild

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// RecordRollups returns the reader of roll-ups over record types
// (TKT-VMSWSWP): every fold goes through the record service, which gates the
// owners before it folds.
//
// Nil: returned when the schema declares no record types.
func RecordRollups(s *Services) visibility.RecordAggregator {
	if svc := Records(s); svc != nil {
		return recordFold{svc: svc}
	}
	return nil
}

// recordFold adapts the record service to [visibility.RecordAggregator].
type recordFold struct{ svc *records.Service }

// FoldRecords implements [visibility.RecordAggregator].
func (f recordFold) FoldRecords(ctx context.Context, q rollup.RecordQuery) (map[string]string, error) {
	if len(q.Owners) == 0 {
		return map[string]string{}, nil // no owners would fold every owner of the type
	}
	points, err := f.svc.Aggregate(ctx, records.AggregateQuery{
		Type: q.Type, Owners: q.Owners, From: q.From, To: q.To, Field: q.Field, PerOwner: true,
	})
	if len(q.Owners) == 1 && errors.Is(err, records.ErrNotFound) {
		// The one named owner is hidden from the caller: it has no rows to show.
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(points))
	for _, p := range points {
		text := p.Sum
		switch q.Func {
		case metamodel.RollupAvg:
			text = p.Avg
		case metamodel.RollupLatestRow:
			text = p.Latest
		case metamodel.RollupCount, metamodel.RollupExists, metamodel.RollupMin, metamodel.RollupMax,
			metamodel.RollupSum, metamodel.RollupLatest, metamodel.RollupBodyTasks:
		}
		if text != "" {
			out[p.Owner] = text
		}
	}
	return out, nil
}
