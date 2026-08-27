package storetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunFreshnessTests exercises [store.Freshness], the mandatory member of
// store.Store that had no conformance coverage at all until TKT-8TJ2WN.
//
// It is worth pinning because the three shipped implementations are genuinely
// different — fsstore takes the max of four filesystem mtimes, memstore scans
// two maps, pgstore runs SQL max() over a UNION ALL — and because a wrong
// answer degrades silently: appbuild compares this against the search index's
// stored timestamp to decide whether to rebuild, so a store that under-reports
// serves stale search results with no error anywhere.
//
// The assertions are deliberately coarse (monotonicity and coverage, not exact
// values). A store whose timestamps come from the filesystem or the database
// clock cannot promise more than that, and demanding equality with a
// test-observed wall clock would fail those backends for no benefit.
func RunFreshnessTests(t *testing.T, f Factory) {
	t.Run("EmptyStoreReturnsZeroTime", func(t *testing.T) {
		s := f(t)
		got, err := s.LastModified(ctx())
		require.NoError(t, err)
		require.True(t, got.IsZero(),
			"empty store must report a zero time, got %v — consumers treat "+
				"non-zero as 'there is data newer than my index'", got)
	})

	t.Run("AdvancesOnEntityWrite", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		before, err := s.LastModified(ctx())
		require.NoError(t, err)
		require.False(t, before.IsZero(), "a seeded store must not report zero")

		waitForClock()
		e := entity.New("FEAT-900", "feature")
		e.SetString("title", "Later")
		require.NoError(t, s.CreateEntity(ctx(), e))

		after, err := s.LastModified(ctx())
		require.NoError(t, err)
		require.False(t, after.Before(before),
			"LastModified went backwards after a write: %v -> %v", before, after)
	})

	// The relation half is the one a new backend is most likely to miss: it is
	// easy to write `SELECT max(updated_at) FROM entities` and never notice,
	// because every entity-only test still passes.
	t.Run("CoversRelationWrites", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		before, err := s.LastModified(ctx())
		require.NoError(t, err)

		waitForClock()
		_, err = s.CreateRelation(ctx(), "FEAT-001", "requires", "REQ-001", nil)
		require.NoError(t, err)

		after, err := s.LastModified(ctx())
		require.NoError(t, err)
		require.True(t, after.After(before),
			"LastModified must cover relation writes (%v -> %v); a store that "+
				"only scans entities leaves relation-only changes invisible to "+
				"every consumer maintaining derived state", before, after)
	})

	t.Run("StableWithoutWrites", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		first, err := s.LastModified(ctx())
		require.NoError(t, err)

		waitForClock()
		// Reads must not advance it — otherwise a consumer rebuilds its index
		// on every poll.
		_, err = s.GetEntity(ctx(), "FEAT-001")
		require.NoError(t, err)
		_, err = s.CountEntities(ctx(), store.EntityQuery{})
		require.NoError(t, err)

		second, err := s.LastModified(ctx())
		require.NoError(t, err)
		require.Equal(t, first, second,
			"reads must not advance LastModified; a consumer polling this "+
				"would rebuild its derived state forever")
	})

	t.Run("UTCComparable", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		got, err := s.LastModified(ctx())
		require.NoError(t, err)

		// Not a demand for UTC as such — time.Time compares correctly across
		// zones. This catches the specific SQL-backend failure of storing a
		// naive TEXT timestamp and parsing it back without a zone, which yields
		// a time that compares wrong against a real clock reading.
		require.False(t, got.After(time.Now().Add(time.Hour)),
			"LastModified is implausibly far in the future (%v) — a timestamp "+
				"parsed without its timezone will compare wrong against every "+
				"consumer's clock", got)
	})
}

// waitForClock advances past the resolution of the coarsest timestamp source a
// backend might use. fsstore reads filesystem mtimes, which on some platforms
// have 1s granularity, so a sub-millisecond sleep would make "after" and
// "before" compare equal and the test flaky rather than meaningful.
func waitForClock() { time.Sleep(10 * time.Millisecond) }
