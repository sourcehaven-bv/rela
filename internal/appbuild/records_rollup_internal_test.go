package appbuild

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const recordRollupMeta = `version: "1.0"
entities:
  site:
    label: Site
    id_prefix: "SITE-"
    properties:
      name: {type: string}
      last_kwh:
        type: decimal
        rollup: {records: reading, latest: kwh}
      week_kwh:
        type: decimal
        rollup: {records: reading, sum: kwh, window: 7d}
      avg_kwh:
        type: decimal
        rollup: {records: reading, avg: kwh}
      n:
        type: integer
        rollup: {records: reading, sum: n}
records:
  reading:
    owner: site
    fields:
      kwh: {type: decimal, scale: 1}
      n: {type: integer}
`

// hiddenOwners reads every owner but the hidden ones, and writes all.
type hiddenOwners map[string]bool

func (h hiddenOwners) Readable(_ context.Context, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = !h[id]
	}
	return out, nil
}

func (hiddenOwners) Writable(_ context.Context, id string) (records.Owner, error) {
	return records.Owner{ID: id, Type: "site", Faces: []map[string]any{{}}}, nil
}

// countingFold counts the folds that reach the record service.
type countingFold struct {
	fold  recordFold
	calls atomic.Int64
}

func (c *countingFold) FoldRecords(ctx context.Context, q rollup.RecordQuery) (map[string]string, error) {
	c.calls.Add(1)
	return c.fold.FoldRecords(ctx, q)
}

func TestRollups_RecordsValues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	meta, err := metamodel.Parse([]byte(recordRollupMeta))
	require.NoError(t, err)
	set, err := rollup.Compile(meta)
	require.NoError(t, err)
	backend, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	svc, err := records.NewService(records.Deps{
		Store: backend, Owners: hiddenOwners{"SITE-3": true}, Meta: meta, Audit: audit.NewMemory(),
		Now: func() time.Time { return now },
	})
	require.NoError(t, err)

	day := 24 * time.Hour
	row := func(owner string, ago time.Duration, kwh string, n int) records.Input {
		return records.Input{Owner: owner, TS: now.Add(-ago), Fields: map[string]any{"kwh": kwh, "n": n}}
	}
	_, err = svc.Append(ctx, "reading", []records.Input{
		row("SITE-1", 20*day, "1.0", 1), row("SITE-1", 3*day, "2.0", 2), row("SITE-1", day, "4.0", 4),
		row("SITE-2", 2*day, "10.5", 7),
		row("SITE-3", day, "99.0", 99),
	})
	require.NoError(t, err)

	agg := &countingFold{fold: recordFold{svc: svc}}
	r, err := visibility.NewRollups(set, memstore.New(), visibility.AllowAllReadQueries{}, visibility.NopRedactor{})
	require.NoError(t, err)
	r = r.WithRecords(agg, func() time.Time { return now })

	ref := func(id string) entity.Ref { return entity.Ref{ID: id, Face: "site"} }
	got := r.Values(ctx, "site", []entity.Ref{ref("SITE-1"), ref("SITE-2"), ref("SITE-3"), ref("SITE-4")},
		store.TrivialScope(), nil)

	require.Equal(t, map[string]any{"last_kwh": "4.0", "week_kwh": "6.0", "avg_kwh": "2.333", "n": int64(7)},
		got["SITE-1"], "latest, windowed sum, average and integer sum")
	require.Equal(t, map[string]any{"last_kwh": "10.5", "week_kwh": "10.5", "avg_kwh": "10.500", "n": int64(7)},
		got["SITE-2"])
	require.NotContains(t, got, "SITE-3", "a hidden owner's rows never contribute")
	require.NotContains(t, got, "SITE-4", "an owner without rows has no value")
	require.Equal(t, int64(4), agg.calls.Load(), "one aggregate call per roll-up column, not per row")

	// A page ten times as long makes the same calls.
	agg.calls.Store(0)
	refs := make([]entity.Ref, 0, 50)
	for i := range 50 {
		refs = append(refs, ref(fmt.Sprintf("SITE-%d", 100+i)))
	}
	r.Values(ctx, "site", refs, store.TrivialScope(), nil)
	require.Equal(t, int64(4), agg.calls.Load())
}

func TestRollups_RecordsWithoutServiceLeftOut(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(recordRollupMeta))
	require.NoError(t, err)
	set, err := rollup.Compile(meta)
	require.NoError(t, err)
	r, err := visibility.NewRollups(set, memstore.New(), visibility.AllowAllReadQueries{}, visibility.NopRedactor{})
	require.NoError(t, err)
	got := r.Values(context.Background(), "site", []entity.Ref{{ID: "SITE-1", Face: "site"}}, store.TrivialScope(), nil)
	require.Empty(t, got)
}

func TestRecordFold_HiddenOwnerAndWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	meta, err := metamodel.Parse([]byte(recordRollupMeta))
	require.NoError(t, err)
	backend, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	require.NoError(t, err)
	svc, err := records.NewService(records.Deps{
		Store: backend, Owners: hiddenOwners{"SITE-3": true}, Meta: meta, Audit: audit.NewMemory(),
		Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	_, err = svc.Append(ctx, "reading", []records.Input{
		{Owner: "SITE-1", TS: now.Add(-time.Hour), Fields: map[string]any{"kwh": "2.0"}},
		{Owner: "SITE-1", TS: now.Add(time.Hour), Fields: map[string]any{"kwh": "9.0"}},
		{Owner: "SITE-3", TS: now.Add(-time.Hour), Fields: map[string]any{"kwh": "5.0"}},
	})
	require.NoError(t, err)
	fold := recordFold{svc: svc}
	q := rollup.RecordQuery{Type: "reading", Field: "kwh", Func: metamodel.RollupLatestRow}

	q.Owners = []string{"SITE-3"}
	got, err := fold.FoldRecords(ctx, q)
	require.NoError(t, err)
	require.Empty(t, got, "a page of one hidden owner has no value, and no error")

	q.Owners = nil
	got, err = fold.FoldRecords(ctx, q)
	require.NoError(t, err)
	require.Empty(t, got, "no owners fold nothing, not every owner")

	q.Owners, q.From, q.To = []string{"SITE-1", "SITE-3"}, now.Add(-24*time.Hour), now
	got, err = fold.FoldRecords(ctx, q)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"SITE-1": "2.0"}, got, "a row dated after the window end does not count")
}
