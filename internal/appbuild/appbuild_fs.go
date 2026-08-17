//go:build !postgres && !memorybackend

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// New builds the services bundle for the default (filesystem) build: an
// fsstore rooted at the project paths plus an in-memory bleve search
// index wired as a write observer. This is the per-scenario recipe — it
// owns only the backend choice; [prepare] and [assemble] do the
// build-agnostic work shared by every build.
func New(cfg Config, opts ...Option) (*Services, error) {
	base, err := prepare(cfg, opts)
	if err != nil {
		return nil, err
	}
	st, searcher, closer, err := openBackend(context.Background(), base)
	if err != nil {
		return nil, err
	}
	// nil VisibleSearcher → assemble derives the generic
	// search.NewVisible wrapper (TKT-BA8BSX); only the postgres
	// recipe wires a native implementation.
	return assemble(base, st, searcher, nil, closer)
}

// openBackend opens the fsstore and the bleve-backed searcher. The bleve
// index is created first and installed as a store observer at open time
// so it receives initial write events; it is then backfilled with
// entities already on disk (the observer is not invoked for those).
//
// A nil index is non-fatal: the store still opens and buildSearcher
// returns an error-Searcher, so read/write paths keep working.
func openBackend(ctx context.Context, base *SharedBase) (store.Store, search.Searcher, io.Closer, error) {
	idx := openSearchIndex(base)

	factory := &app.FSFactory{FS: base.cfg.FS, Paths: base.cfg.Paths}
	if idx != nil {
		factory.AddObserver(idx)
	}
	st, err := factory.OpenStore(base.meta)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open store: %w", err)
	}

	if idx == nil {
		return st, search.ErrSearcher(errors.New("search index not available")), noopCloser{}, nil
	}
	if err := backfillBleve(ctx, idx, st); err != nil {
		slog.Warn("appbuild: failed to index entities", "error", err)
	}
	return st, search.New(st, idx), idx, nil
}

// searchIndexDir is the directory under the project cache dir holding the
// persistent search index.
const searchIndexDir = "search"

// openSearchIndex opens the on-disk search index, falling back to an
// in-memory one when no cache dir is configured or the on-disk index is
// unavailable. Returns nil only when neither could be built — the caller
// treats that as "search unavailable" rather than a startup failure.
//
// The on-disk index is preferred because it both survives restarts (see
// backfillBleve, which skips reindexing an already-current index) and
// keeps scorch's persister/merger running, without which per-write
// segments accumulate for the life of the process. The in-memory fallback
// reinstates that growth, so it is a degraded mode, not an equivalent one.
func openSearchIndex(base *SharedBase) *bleveindex.Index {
	if base.cfg.Paths != nil && base.cfg.Paths.CacheDir != "" {
		path := filepath.Join(base.cfg.Paths.CacheDir, searchIndexDir)
		idx, err := bleveindex.New(path)
		if err == nil {
			return idx
		}
		// Most likely another rela process already holds this project's
		// index. Falling back keeps this process fully functional.
		slog.Warn("appbuild: on-disk search index unavailable, using in-memory index",
			"path", path, "error", err)
	}

	idx, err := bleveindex.NewMem()
	if err != nil {
		slog.Warn("appbuild: search index unavailable", "error", err)
		return nil
	}
	return idx
}

// backfillChunkSize bounds how many entities are held in memory — and
// pushed into a single bleve batch — at a time during backfill. Indexing
// every entity in one batch made peak RSS scale with the whole corpus
// (~1.1GB for 2.4k entities); chunking lets the GC reclaim each batch's
// index structures before the next one is built.
const backfillChunkSize = 100

// backfillBleve indexes every entity currently in the store, streaming
// them through fixed-size batches rather than materializing the whole
// corpus. Per-entity and list errors are collected and returned together
// so the caller logs a complete picture rather than swallowing failures.
//
// A persistent index that already covers the store's newest mutation is
// left alone — see [indexIsCurrent].
func backfillBleve(ctx context.Context, idx *bleveindex.Index, st store.Store) error {
	if indexIsCurrent(ctx, idx, st) {
		slog.Debug("appbuild: search index is current, skipping backfill")
		return nil
	}
	// Sampled before indexing starts: anything written while the backfill
	// runs must leave the store looking newer than this, so the next
	// startup reindexes instead of trusting a partially-covered index.
	storeMtime, mtimeErr := st.LastModified(ctx)
	if mtimeErr != nil {
		storeMtime = time.Time{}
	}

	if err := backfillBleveInto(ctx, idx, st); err != nil {
		return err
	}
	// Only a complete, clean backfill earns a watermark. After a partial
	// one the index does not cover the store, so leaving the watermark
	// unset makes the next startup rebuild it.
	markIndexBackfilled(idx, storeMtime)
	return nil
}

// entityBatchIndexer is the slice of the index that the backfill loop
// needs. Declaring it here rather than taking *bleveindex.Index lets tests
// observe the batching itself — the property chunking exists to provide,
// which an end-state assertion cannot distinguish from one giant batch.
type entityBatchIndexer interface {
	IndexBatch(entities []*entity.Entity) (int, error)
}

// backfillBleveInto streams every entity in the store into idx through
// fixed-size batches. It is the loop half of [backfillBleve], split out so
// the batching can be tested directly.
func backfillBleveInto(ctx context.Context, idx entityBatchIndexer, st store.Store) error {
	var (
		chunk    = make([]*entity.Entity, 0, backfillChunkSize)
		listErrs []error
		total    int
		indexed  int
		indexErr error
	)
	flush := func() {
		if len(chunk) == 0 {
			return
		}
		n, err := idx.IndexBatch(chunk)
		indexed += n
		if err != nil {
			indexErr = err
		}
		// Release the entities so the GC can reclaim them (and the bleve
		// index structures they fed) before the next chunk is read.
		clear(chunk)
		chunk = chunk[:0]
	}

	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			listErrs = append(listErrs, err)
			continue
		}
		if e == nil {
			// Defensive: the iterator contract is (nil, err) on failure,
			// but a nil entity with a nil error would panic in the
			// indexer — during startup, where there is no recover.
			continue
		}
		chunk = append(chunk, e)
		total++
		if len(chunk) == backfillChunkSize {
			flush()
			// Stop at the first index error rather than reading the rest
			// of the corpus only to count it: continuing would leave the
			// chunk undrained and growing, reinstating the unbounded
			// memory use chunking exists to prevent.
			if indexErr != nil {
				break
			}
		}
	}
	if indexErr == nil {
		flush()
	}

	if len(listErrs) == 0 && indexErr == nil {
		return nil
	}
	// "read but not indexed" — entities the iterator yielded that did not
	// make it into the index, whether the batch rejected them or the
	// backfill stopped early. Entities never reached because of that
	// early stop are not counted here; total only counts what was read.
	skipped := total - indexed
	return fmt.Errorf("backfill indexed %d entities, skipped %d, list errors: %v, index error: %w",
		indexed, skipped, listErrs, indexErr)
}

// indexIsCurrent reports whether idx already reflects every mutation the
// store knows about, so the backfill can be skipped entirely.
//
// The comparison is against the watermark [markIndexBackfilled] stamped at
// the end of the last backfill, NOT against [bleveindex.Index.LastModified].
// The two measure different clocks: LastModified tracks entity.UpdatedAt
// (set in memory before the write), while the store reports the newest
// file mtime on disk (set when that write lands). The store's value is
// therefore always a few milliseconds ahead, and comparing the two would
// report "stale" on an index that is in fact perfectly current — which is
// exactly what happened before this was stamped explicitly.
//
// This is a startup optimization, so it fails closed: any error, a missing
// or zero watermark, or an empty index means "reindex". A stale index that
// wrongly claimed to be current would serve wrong results indefinitely,
// whereas a needless reindex only costs time.
func indexIsCurrent(ctx context.Context, idx *bleveindex.Index, st store.Store) bool {
	indexed, err := idx.Watermark(backfillWatermarkKey)
	if err != nil || indexed.IsZero() {
		return false
	}
	stored, err := st.LastModified(ctx)
	if err != nil || stored.IsZero() {
		return false
	}
	if stored.After(indexed) {
		return false
	}
	// A watermark can survive in an index whose documents did not (a
	// half-written or truncated index directory), so require that the
	// index actually holds something before trusting it.
	count, err := idx.DocCount()
	return err == nil && count > 0
}

// backfillWatermarkKey names the stored store-mtime watermark. It is
// namespaced separately from the index's own LastModified so the two
// clocks (see [indexIsCurrent]) can never be confused for one another.
const backfillWatermarkKey = "rela:backfill_store_mtime"

// markIndexBackfilled records the store mtime that the just-completed
// backfill covers, so a later startup can skip reindexing.
//
// The value is read BEFORE the backfill by the caller and written after,
// which is the conservative order: any write landing during the backfill
// leaves the store newer than the recorded watermark, so the next startup
// reindexes rather than trusting a partially-covered index.
func markIndexBackfilled(idx *bleveindex.Index, storeMtime time.Time) {
	if storeMtime.IsZero() {
		return
	}
	if err := idx.SetWatermark(backfillWatermarkKey, storeMtime); err != nil {
		slog.Warn("appbuild: could not record search index watermark", "error", err)
	}
}

// noopCloser is returned when no closable search resource is held.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }
