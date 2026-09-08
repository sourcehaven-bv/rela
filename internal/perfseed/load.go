package perfseed

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LoadOptions tunes [Load].
type LoadOptions struct {
	// BatchSize is the number of writes per store.Tx. Zero means 500. On
	// the postgres backend a Tx holds the schema-wide write lock, so a
	// batch bounds how long other writers wait; on fs/mem it is a mutex
	// and the size only affects progress granularity.
	BatchSize int
	// Progress, when set, is called after each committed batch.
	Progress func(Summary)
}

// Summary counts what a load wrote.
type Summary struct {
	Entities  int
	Relations int
	Elapsed   time.Duration
}

// Load writes every entity and relation g produces into st, in batches of
// one store.Tx each. It writes through the raw store on purpose — see the
// package doc — and expects the caller to have stamped attribution on ctx.
//
// A batch that fails stops the load; earlier batches stay committed. Re-run
// on an empty store rather than resuming: the generator is deterministic,
// and `rela dev seed` refuses a non-empty store for exactly that reason.
func Load(ctx context.Context, st store.Store, g *Generator, opts LoadOptions) (Summary, error) {
	batch := opts.BatchSize
	if batch <= 0 {
		batch = 500
	}
	start := time.Now()
	var sum Summary
	report := func() {
		sum.Elapsed = time.Since(start)
		if opts.Progress != nil {
			opts.Progress(sum)
		}
	}

	err := inBatches(ctx, st, batch, g.Entities(), func(view store.Store, e *entity.Entity) error {
		if err := view.CreateEntity(ctx, e); err != nil {
			return fmt.Errorf("create %s@%s: %w", e.ID, e.Face, err)
		}
		return nil
	}, func(n int) { sum.Entities += n; report() })
	if err != nil {
		return sum, err
	}
	err = inBatches(ctx, st, batch, g.Relations(), func(view store.Store, r Relation) error {
		data := &store.RelationData{FromFace: r.FromFace}
		if _, cerr := view.CreateRelation(ctx, r.From, r.Type, r.To, data); cerr != nil {
			return fmt.Errorf("create %s --%s--> %s: %w", r.From, r.Type, r.To, cerr)
		}
		return nil
	}, func(n int) { sum.Relations += n; report() })
	if err != nil {
		return sum, err
	}
	sum.Elapsed = time.Since(start)
	return sum, nil
}

// inBatches drains seq into st, `size` items per Tx, calling write for each
// item inside the transaction and done(n) after each commit. It stops at
// the first error or when ctx is cancelled.
func inBatches[T any](
	ctx context.Context, st store.Store, size int, seq iter.Seq[T],
	write func(view store.Store, item T) error, done func(n int),
) error {
	buf := make([]T, 0, size)
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		err := st.Tx(ctx, func(view store.Store) error {
			for _, item := range buf {
				if err := write(view, item); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		done(len(buf))
		buf = buf[:0]
		return nil
	}
	for item := range seq {
		if err := ctx.Err(); err != nil {
			return err
		}
		buf = append(buf, item)
		if len(buf) == size {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}
