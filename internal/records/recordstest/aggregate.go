package recordstest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/records"
)

// testAggregateMatchesGeneric holds a native [records.Aggregator] to the
// result of [records.AggregateGeneric] over every bucket and owner mode.
func testAggregateMatchesGeneric(t *testing.T, s records.Store) {
	t.Helper()
	agg, ok := s.(records.Aggregator)
	if !ok {
		t.Skip("the store is not a records.Aggregator")
	}
	const (
		day  = 24 * time.Hour
		far  = 40 * day
		half = 30 * time.Minute
	)
	appendRows(t, s,
		Row("a1", "meter", "A", -far, map[string]any{"kwh": "1.50", "n": int64(2)}),
		Row("a2", "meter", "A", 0, map[string]any{"kwh": "2.25", "n": int64(3)}),
		Row("a3", "meter", "A", half, map[string]any{"kwh": "0.10"}),
		Row("a4", "meter", "A", 3*day, map[string]any{"note": "no number"}),
		Row("b1", "meter", "B", time.Hour, map[string]any{"kwh": "-4", "n": int64(-1)}),
		Row("b2", "meter", "B", far, map[string]any{"kwh": "7.005", "n": int64(3)}),
		Row("b3", "meter", "B", far, map[string]any{"kwh": "7.005", "n": int64(3)}),
		Row("c1", "meter", "C", 2*day, map[string]any{"kwh": "5.00"}),
		Row("x1", "other", "A", 0, map[string]any{"kwh": "9"}),
	)
	buckets := []records.Bucket{records.BucketNone, records.BucketHour, records.BucketDay,
		records.BucketWeek, records.BucketMonth}
	fields := []struct {
		name  string
		scale int
	}{{"", 0}, {"kwh", -1}, {"kwh", 3}, {"n", 0}}
	ranges := []struct{ from, to time.Time }{
		{}, {T0.Add(-time.Hour), T0.Add(3 * day)}, {T0, time.Time{}}, {time.Time{}, T0},
	}
	owners := [][]string{{"A"}, {"A", "B", "C"}, {"C", "B"}}
	ctx := context.Background()
	for _, b := range buckets {
		for _, f := range fields {
			for _, per := range []bool{false, true} {
				for _, r := range ranges {
					for _, o := range owners {
						q := records.AggregateQuery{Type: "meter", Owners: o, From: r.from, To: r.to,
							Field: f.name, Bucket: b, PerOwner: per}
						want, err := records.AggregateGeneric(ctx, s, q, f.scale)
						require.NoError(t, err)
						got, err := agg.Aggregate(ctx, q, f.scale)
						require.NoError(t, err)
						require.Equal(t, want, got, "%+v", q)
					}
				}
			}
		}
	}
}
