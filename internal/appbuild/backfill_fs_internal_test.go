//go:build !postgres && !memorybackend

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// seedEntities creates n entities sharing a searchable marker word so a
// single query can count everything that reached the index.
func seedEntities(t *testing.T, st store.Store, n int) {
	t.Helper()
	ctx := context.Background()
	for i := range n {
		e := &entity.Entity{
			ID:   fmt.Sprintf("TKT-%05d", i),
			Type: "ticket",
			Properties: map[string]any{
				"title": fmt.Sprintf("backfillmarker entity %d", i),
			},
		}
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed entity %d: %v", i, err)
		}
	}
}

// indexedCount reports how many of the seeded entities the index can
// return. The limit is generous so the query never truncates the answer.
func indexedCount(t *testing.T, idx *bleveindex.Index, limit int) int {
	t.Helper()
	ids, err := idx.Search("backfillmarker", limit)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return len(ids)
}

// backfillBleve streams entities through fixed-size batches rather than
// pushing the whole corpus into one bleve batch (see backfillChunkSize).
// The chunking arithmetic has boundaries worth pinning — an empty store, a
// corpus that is an exact multiple of the chunk size (where a naive
// trailing flush would emit an empty batch), and corpora that straddle a
// chunk. Each must index every entity exactly once, so these assert on how
// many entities the index can return rather than on how many batches were
// issued: that is the property callers depend on, and it stays true if the
// chunk size is ever retuned.
func TestBackfillBleve_IndexesEveryEntityAcrossChunkBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
	}{
		{name: "empty store", count: 0},
		{name: "single entity", count: 1},
		{name: "one under a chunk", count: backfillChunkSize - 1},
		{name: "exactly one chunk", count: backfillChunkSize},
		{name: "one over a chunk", count: backfillChunkSize + 1},
		{name: "exact multiple of chunk", count: backfillChunkSize * 3},
		{name: "straddles several chunks", count: backfillChunkSize*2 + 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			st := memstore.New()
			seedEntities(t, st, tc.count)

			idx, err := bleveindex.NewMem()
			if err != nil {
				t.Fatalf("new index: %v", err)
			}
			t.Cleanup(func() { _ = idx.Close() })

			if err := backfillBleve(context.Background(), idx, st); err != nil {
				t.Fatalf("backfillBleve: %v", err)
			}

			if got := indexedCount(t, idx, tc.count+10); got != tc.count {
				t.Errorf("indexed %d entities, want %d", got, tc.count)
			}
		})
	}
}

// A store that yields an error part-way through must not abort the
// backfill: the error is collected and reported, while every entity that
// did read successfully is still indexed. The reported counts are derived
// from a running total rather than len(entities), so this also pins that
// the chunked rewrite kept the accounting honest.
func TestBackfillBleve_ReportsListErrorsAndIndexesTheRest(t *testing.T) {
	t.Parallel()

	const good = backfillChunkSize + 5
	backing := memstore.New()
	seedEntities(t, backing, good)

	idx, err := bleveindex.NewMem()
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })

	// Inject one unreadable entry mid-stream, before the first chunk
	// boundary, so the failure lands inside a chunk rather than between two.
	st := &erroringLister{Store: backing, failAt: backfillChunkSize / 2}

	err = backfillBleve(context.Background(), idx, st)
	if err == nil {
		t.Fatal("expected an error describing the failed list entries")
	}
	if !strings.Contains(err.Error(), "list errors") {
		t.Errorf("error %q does not mention the list errors", err)
	}

	// The bad entry is skipped; every good entity is still indexed.
	if got := indexedCount(t, idx, good+10); got != good {
		t.Errorf("indexed %d entities, want %d", got, good)
	}
}

// The whole point of chunking is that no single bleve batch — and so no
// single peak in memory — scales with the corpus. Asserting only that
// every entity ends up indexed does not pin that: the original
// one-giant-batch implementation satisfies it too. This counts the batches
// and their sizes instead, so removing the chunk boundary fails here.
func TestBackfillBleve_BoundsBatchSize(t *testing.T) {
	t.Parallel()

	const count = backfillChunkSize*3 + 17
	st := memstore.New()
	seedEntities(t, st, count)

	idx, err := bleveindex.NewMem()
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })

	spy := &batchSpy{Index: idx}
	if err := backfillBleveInto(context.Background(), spy, st); err != nil {
		t.Fatalf("backfillBleve: %v", err)
	}

	if spy.largest > backfillChunkSize {
		t.Errorf("largest batch was %d entities, must not exceed chunk size %d",
			spy.largest, backfillChunkSize)
	}
	wantBatches := (count + backfillChunkSize - 1) / backfillChunkSize
	if spy.batches != wantBatches {
		t.Errorf("issued %d batches for %d entities, want %d",
			spy.batches, count, wantBatches)
	}
	if spy.total != count {
		t.Errorf("batched %d entities in total, want %d", spy.total, count)
	}
}

// An index error must stop the backfill rather than let it read the rest
// of the corpus purely to count it — continuing leaves the pending chunk
// undrained and growing, which is the unbounded memory use chunking is
// meant to prevent.
func TestBackfillBleve_StopsReadingAfterIndexError(t *testing.T) {
	t.Parallel()

	const count = backfillChunkSize * 10
	backing := memstore.New()
	seedEntities(t, backing, count)

	idx, err := bleveindex.NewMem()
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })

	counter := &countingLister{Store: backing}
	// Fail on the second batch, so one chunk succeeds first.
	spy := &batchSpy{Index: idx, failOnBatch: 2}

	if err := backfillBleveInto(context.Background(), spy, counter); err == nil {
		t.Fatal("expected the index error to be reported")
	}

	// Reading must stop near the failure, not run to the end of the corpus.
	if counter.yielded > backfillChunkSize*3 {
		t.Errorf("read %d entities after an index error; expected to stop "+
			"shortly after the failing batch (corpus is %d)", counter.yielded, count)
	}
}

// A persisted index is only reusable if the watermark it was stamped with
// still covers the store. These pin the decision itself, because getting
// it wrong in the permissive direction is silent: the process would keep
// serving results from an index missing everything edited while it was
// down.
func TestIndexIsCurrent(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name      string
		watermark time.Time // zero = never stamped
		storeTime time.Time
		storeErr  error
		seed      int
		want      bool
	}{
		{
			name:      "store unchanged since backfill",
			watermark: future, storeTime: past, seed: 3, want: true,
		},
		{
			name:      "store changed after backfill",
			watermark: past, storeTime: future, seed: 3, want: false,
		},
		{
			name:      "never backfilled",
			watermark: time.Time{}, storeTime: past, seed: 3, want: false,
		},
		{
			name:      "watermark present but index is empty",
			watermark: future, storeTime: past, seed: 0, want: false,
		},
		{
			name:      "store mtime unavailable",
			watermark: future, storeErr: errors.New("stat failed"), seed: 3, want: false,
		},
		{
			name:      "store reports zero mtime",
			watermark: future, storeTime: time.Time{}, seed: 3, want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			idx, err := bleveindex.NewMem()
			if err != nil {
				t.Fatalf("new index: %v", err)
			}
			t.Cleanup(func() { _ = idx.Close() })

			backing := memstore.New()
			seedEntities(t, backing, tc.seed)
			if tc.seed > 0 {
				if err := backfillBleveInto(context.Background(), idx, backing); err != nil {
					t.Fatalf("seed index: %v", err)
				}
			}
			if !tc.watermark.IsZero() {
				if err := idx.SetWatermark(backfillWatermarkKey, tc.watermark); err != nil {
					t.Fatalf("set watermark: %v", err)
				}
			}

			st := &fixedMtimeStore{Store: backing, mtime: tc.storeTime, err: tc.storeErr}
			if got := indexIsCurrent(context.Background(), idx, st); got != tc.want {
				t.Errorf("indexIsCurrent = %v, want %v", got, tc.want)
			}
		})
	}
}

// A backfill that did not complete cleanly must not leave a watermark
// behind: doing so would tell the next startup that a partial index was
// complete, and the entities missed would stay missing.
func TestBackfillBleve_NoWatermarkAfterFailedBackfill(t *testing.T) {
	t.Parallel()

	backing := memstore.New()
	seedEntities(t, backing, backfillChunkSize+5)
	st := &erroringLister{Store: backing, failAt: 3}

	idx, err := bleveindex.NewMem()
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	t.Cleanup(func() { _ = idx.Close() })

	if backfillErr := backfillBleve(context.Background(), idx, st); backfillErr == nil {
		t.Fatal("expected the list error to be reported")
	}

	got, err := idx.Watermark(backfillWatermarkKey)
	if err != nil {
		t.Fatalf("read watermark: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("watermark %v was recorded after a failed backfill; want none", got)
	}
}

// fixedMtimeStore reports a caller-chosen LastModified so the reuse
// decision can be exercised without touching real file timestamps.
type fixedMtimeStore struct {
	store.Store
	mtime time.Time
	err   error
}

func (f *fixedMtimeStore) LastModified(context.Context) (time.Time, error) {
	return f.mtime, f.err
}

// batchSpy wraps an index and records how many entities each IndexBatch
// call received, so tests can assert on batching rather than only on the
// end state.
type batchSpy struct {
	*bleveindex.Index
	failOnBatch int // 1-based; 0 disables

	batches int
	largest int
	total   int
}

func (s *batchSpy) IndexBatch(entities []*entity.Entity) (int, error) {
	s.batches++
	s.total += len(entities)
	if len(entities) > s.largest {
		s.largest = len(entities)
	}
	if s.failOnBatch != 0 && s.batches == s.failOnBatch {
		return 0, errors.New("synthetic index failure")
	}
	return s.Index.IndexBatch(entities)
}

// countingLister counts how many entities the backfill actually read.
type countingLister struct {
	store.Store
	yielded int
}

func (c *countingLister) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for ent, err := range c.Store.ListEntities(ctx, q) {
			c.yielded++
			if !yield(ent, err) {
				return
			}
		}
	}
}

// erroringLister wraps a store and injects a single list error at a chosen
// position, standing in for an entity file that fails to parse.
type erroringLister struct {
	store.Store
	failAt int
}

func (e *erroringLister) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		i := 0
		for ent, err := range e.Store.ListEntities(ctx, q) {
			if i == e.failAt {
				i++
				if !yield(nil, errors.New("failed to parse frontmatter")) {
					return
				}
			}
			i++
			if !yield(ent, err) {
				return
			}
		}
	}
}
