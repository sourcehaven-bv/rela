package storetest_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// recorders lists the three read recorders, each wrapping a fresh memstore.
func recorders() []struct {
	name string
	wrap func(store.Store) store.Store
} {
	return []struct {
		name string
		wrap func(store.Store) store.Store
	}{
		{"Counting", func(s store.Store) store.Store { return storetest.NewCounting(s) }},
		{"Breadth", func(s store.Store) store.Store { return storetest.NewBreadth(s) }},
		{"BodyWatch", func(s store.Store) store.Store { return storetest.NewBodyWatch(s) }},
	}
}

// hidesCapabilities embeds only store.Store, so a soft-delete capability of
// the store it wraps is not promoted.
type hidesCapabilities struct{ store.Store }

func seedDoc(ctx context.Context, t *testing.T, s store.Store, id string) {
	t.Helper()
	require.NoError(t, s.CreateEntity(ctx, &entity.Entity{
		ID: id, Type: "doc", Properties: map[string]any{"title": id},
	}))
}

// TestRecorders_ForwardSoftDelete pins that a recorder keeps the wrapped
// store's soft-delete capability, outside and inside a transaction, so a test
// that stacks recorders still takes the production delete path.
func TestRecorders_ForwardSoftDelete(t *testing.T) {
	for _, tc := range recorders() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := tc.wrap(memstore.New())
			seedDoc(ctx, t, s, "DOC-1")
			seedDoc(ctx, t, s, "DOC-2")

			sd, ok := s.(store.SoftDeleteProvider)
			require.True(t, ok, "%s hides store.SoftDeleteProvider", tc.name)
			_, err := sd.SoftDelete().MarkDeleted(ctx, "DOC-1", "alice")
			require.NoError(t, err)

			// Inside a transaction the view must forward too: memstore's
			// parent soft deleter would wait on the lock the open Tx holds.
			require.NoError(t, s.Tx(ctx, func(view store.Store) error {
				vsd, ok := view.(store.SoftDeleteProvider)
				require.True(t, ok, "%s transaction view hides store.SoftDeleteProvider", tc.name)
				_, txErr := vsd.SoftDelete().MarkDeleted(ctx, "DOC-2", "alice")
				return txErr
			}))

			marked, err := sd.SoftDelete().ListMarked(ctx)
			require.NoError(t, err)
			ids := make([]string, 0, len(marked))
			for _, m := range marked {
				ids = append(ids, m.ID)
			}
			assert.ElementsMatch(t, []string{"DOC-1", "DOC-2"}, ids)
			_, err = s.GetEntity(ctx, entity.Ref{ID: "DOC-1"})
			assert.ErrorIs(t, err, store.ErrNotFound, "a marked entity reads as missing")
		})
	}
}

// TestRecorders_SoftDeleteWithoutCapabilityFails pins that a recorder over a
// store with no soft delete fails the call rather than reporting success.
func TestRecorders_SoftDeleteWithoutCapabilityFails(t *testing.T) {
	for _, tc := range recorders() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := tc.wrap(hidesCapabilities{memstore.New()})
			seedDoc(ctx, t, s, "DOC-1")
			sd := s.(store.SoftDeleteProvider).SoftDelete()

			_, err := sd.MarkDeleted(ctx, "DOC-1", "alice")
			require.Error(t, err)
			_, err = sd.Unmark(ctx, "DOC-1")
			require.Error(t, err)
			_, err = sd.ListMarked(ctx)
			require.Error(t, err)
			_, err = sd.PurgeMarked(ctx, "DOC-1")
			require.Error(t, err)

			_, err = s.GetEntity(ctx, entity.Ref{ID: "DOC-1"})
			assert.NoError(t, err, "a failed soft delete leaves the entity in place")
		})
	}
}

// TestRecorders_ForwardMentionIndex pins that a recorder keeps the wrapped
// store's mention index, outside and inside a transaction, so backlink reads
// behind a stack of recorders still reach it.
func TestRecorders_ForwardMentionIndex(t *testing.T) {
	for _, tc := range recorders() {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := tc.wrap(memstore.New())
			idx, ok := s.(store.MentionIndex)
			require.True(t, ok, "%s hides store.MentionIndex", tc.name)
			_, err := idx.MentionSources(ctx, "DOC-1", entity.Ref{}, 10)
			require.NoError(t, err)

			require.NoError(t, s.Tx(ctx, func(view store.Store) error {
				vidx, ok := view.(store.MentionIndex)
				require.True(t, ok, "%s transaction view hides store.MentionIndex", tc.name)
				_, txErr := vidx.MentionSources(ctx, "DOC-1", entity.Ref{}, 10)
				return txErr
			}))

			_, err = tc.wrap(hidesCapabilities{memstore.New()}).(store.MentionIndex).
				MentionSources(ctx, "DOC-1", entity.Ref{}, 10)
			require.Error(t, err, "a store with no mention index must fail, not answer empty")
		})
	}
}

// TestRecorders_UnwrapToBackend pins that a stack of recorders unwraps to
// the store it records, so a lookup keyed on the backend type (the database
// handle piles and state share with pgstore) still finds it.
func TestRecorders_UnwrapToBackend(t *testing.T) {
	backend := memstore.New()
	var s store.Store = backend
	for _, tc := range recorders() {
		s = tc.wrap(s)
	}
	assert.Same(t, backend, store.Innermost(s))
}

// TestCounting_TxSharesCounters pins that reads inside a transaction land in
// the parent's counters under the parent's lock. Run under -race: a view with
// its own mutex over the parent's map is a data race against a concurrent
// read outside the transaction.
func TestCounting_TxSharesCounters(t *testing.T) {
	ctx := context.Background()
	c := storetest.NewCounting(memstore.New())
	seedDoc(ctx, t, c, "DOC-1")
	const outside, inside = 50, 50

	var wg sync.WaitGroup
	wg.Go(func() {
		for range outside {
			_, _ = c.GetEntity(ctx, entity.Ref{ID: "DOC-1"})
		}
	})
	require.NoError(t, c.Tx(ctx, func(view store.Store) error {
		for range inside {
			if _, err := view.GetEntity(ctx, entity.Ref{ID: "DOC-1"}); err != nil {
				return err
			}
		}
		return nil
	}))
	wg.Wait()

	assert.Equal(t, outside+inside, c.Calls()["GetEntity"], c.String())

	c.Reset()
	require.NoError(t, c.Tx(ctx, func(view store.Store) error {
		_, err := view.GetEntity(ctx, entity.Ref{ID: "DOC-1"})
		return err
	}))
	assert.Equal(t, 1, c.Reads(), "a view opened after Reset counts into the cleared counters")
}
