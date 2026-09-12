package sqlitestore_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// fixedProjection is a ProjectionProvider with a constant schema, so a capture
// is never skipped for want of one.
type fixedProjection struct{}

func (fixedProjection) Projection() (string, []byte) { return "schema-1", []byte(`{"v":1}`) }

// immediateSweep captures anything at all, with no debounce, so a test does not
// have to wait out a settle window.
func immediateSweep(batch int) store.SweepConfig {
	return store.SweepConfig{
		Interval:     time.Hour, // never auto-fires; tests drive ticks by hand
		Idle:         time.Nanosecond,
		MaxStaleness: time.Nanosecond,
		Batch:        batch,
	}
}

// TestSweepDrainsABacklogLargerThanOneBatch is the starvation regression.
//
// The sweep dedups in Go — captureOne compares content hashes and writes
// nothing when they match. So if the candidate query selects rows that are
// already captured and unchanged, those rows fill the batch on EVERY tick and
// the rows behind them are never reached. With `ORDER BY updated_at ASC` the
// same first Batch rows come back forever: not slow, never.
//
// The test therefore uses a backlog several times Batch and asserts every
// entity ends up with a version, having driven only a handful of ticks.
func TestSweepDrainsABacklogLargerThanOneBatch(t *testing.T) {
	const (
		total = 12
		batch = 3
	)
	s := open(t)
	ctx := t.Context()

	for i := range total {
		e := entity.New(fmt.Sprintf("FEAT-%02d", i), "feature")
		e.SetString("title", fmt.Sprintf("entity %d", i))
		require.NoError(t, s.CreateEntity(ctx, e))
	}

	// Ceil(total/batch) ticks would just suffice if every tick makes progress;
	// allow a couple spare so the assertion is about draining rather than about
	// hitting an exact tick count.
	for range (total / batch) + 2 {
		require.NoError(t, s.SweepNow(ctx, fixedProjection{}, immediateSweep(batch)))
	}

	svc := s.VersionStore()
	var missing []string
	for i := range total {
		id := fmt.Sprintf("FEAT-%02d", i)
		versions, err := svc.ListVersions(ctx, id)
		require.NoError(t, err)
		if len(versions) == 0 {
			missing = append(missing, id)
		}
	}
	require.Emptyf(t, missing,
		"%d of %d entities never got a version: the sweep re-selects already-captured "+
			"rows, so they fill every batch and the backlog behind them starves",
		len(missing), total)
}

// TestSweepIsIdleWhenNothingChanged pins the other half: once everything is
// captured, a tick must find no candidates at all.
//
// Without this, the drain fix above could be satisfied by a query that simply
// returns everything every time — correct but pointless, and quadratic in the
// number of entities per tick.
func TestSweepIsIdleWhenNothingChanged(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	e := entity.New("FEAT-1", "feature")
	e.SetString("title", "a feature")
	require.NoError(t, s.CreateEntity(ctx, e))

	require.NoError(t, s.SweepNow(ctx, fixedProjection{}, immediateSweep(10)))
	svc := s.VersionStore()
	first, err := svc.ListVersions(ctx, "FEAT-1")
	require.NoError(t, err)
	require.Len(t, first, 1, "the first tick must capture the new entity")

	// Several more ticks over unchanged content must add nothing. The Go-side
	// dedup would also produce this, so the assertion is about the count
	// staying put; the query-level gate is what makes it cheap.
	for range 3 {
		require.NoError(t, s.SweepNow(ctx, fixedProjection{}, immediateSweep(10)))
	}
	after, err := svc.ListVersions(ctx, "FEAT-1")
	require.NoError(t, err)
	require.Len(t, after, 1, "an unchanged entity was captured more than once")
}

// TestSweepCapturesAnEditAfterCapture is the guard against over-correcting:
// the dirty gate must not make a genuinely edited entity invisible.
func TestSweepCapturesAnEditAfterCapture(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	e := entity.New("FEAT-1", "feature")
	e.SetString("title", "before")
	require.NoError(t, s.CreateEntity(ctx, e))
	require.NoError(t, s.SweepNow(ctx, fixedProjection{}, immediateSweep(10)))

	updated, err := s.GetEntity(ctx, "FEAT-1")
	require.NoError(t, err)
	updated.SetString("title", "after")
	require.NoError(t, s.UpdateEntity(ctx, updated))

	require.NoError(t, s.SweepNow(ctx, fixedProjection{}, immediateSweep(10)))

	versions, err := s.VersionStore().ListVersions(ctx, "FEAT-1")
	require.NoError(t, err)
	require.Len(t, versions, 2,
		"the edit was not captured: the dirty gate is excluding rows that changed")
}
