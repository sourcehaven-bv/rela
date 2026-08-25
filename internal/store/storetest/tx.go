package storetest

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunTxTests exercises the store.Transactor contract every backend must
// meet: serialization, view re-entrancy, read-your-writes, error
// propagation, and nested-Tx joining. Rollback and post-commit event
// delivery are backend-dependent — see RunTxRollbackTests.
func RunTxTests(t *testing.T, f Factory) {
	t.Run("WriteVisibleAfterTx", func(t *testing.T) {
		s := f(t)
		err := s.Tx(ctx(), func(tx store.Store) error {
			e := entity.New("FEAT-100", "feature")
			e.SetString("title", "In transaction")
			return tx.CreateEntity(ctx(), e)
		})
		require.NoError(t, err)

		got, err := s.GetEntity(ctx(), "FEAT-100")
		require.NoError(t, err)
		require.Equal(t, "In transaction", got.GetString("title"))
	})

	t.Run("ReadYourWrites", func(t *testing.T) {
		s := f(t)
		err := s.Tx(ctx(), func(tx store.Store) error {
			e := entity.New("FEAT-100", "feature")
			e.SetString("title", "Visible inside")
			if err := tx.CreateEntity(ctx(), e); err != nil {
				return err
			}
			got, err := tx.GetEntity(ctx(), "FEAT-100")
			if err != nil {
				return err
			}
			if got.GetString("title") != "Visible inside" {
				return errors.New("tx read did not observe tx write")
			}
			if _, err := tx.GetEntity(ctx(), "FEAT-404"); !errors.Is(err, store.ErrNotFound) {
				return errors.New("missing entity should be ErrNotFound inside tx")
			}
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("SerializedReadModifyWrite", func(t *testing.T) {
		// The lost-update check: N concurrent transactions each do a
		// read-modify-write of the same counter property. Serialized
		// transactions end at exactly N; interleaved ones lose updates.
		// As a regression detector this is probabilistic (a lucky
		// scheduler could serialize broken code by chance), which is
		// inherent to lost-update tests — a green run is strong
		// evidence, not proof; a red run is always a real bug.
		s := f(t)
		seed := entity.New("FEAT-001", "feature")
		seed.SetString("n", "0")
		require.NoError(t, s.CreateEntity(ctx(), seed))

		const writers = 8
		var wg sync.WaitGroup
		errs := make([]error, writers)
		for i := range writers {
			wg.Go(func() {
				errs[i] = s.Tx(ctx(), func(tx store.Store) error {
					e, err := tx.GetEntity(ctx(), "FEAT-001")
					if err != nil {
						return err
					}
					n, err := strconv.Atoi(e.GetString("n"))
					if err != nil {
						return err
					}
					runtime.Gosched() // widen the read-modify-write window
					e.SetString("n", strconv.Itoa(n+1))
					return tx.UpdateEntity(ctx(), e)
				})
			})
		}
		wg.Wait()
		for i, err := range errs {
			require.NoError(t, err, "writer %d", i)
		}

		got, err := s.GetEntity(ctx(), "FEAT-001")
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(writers), got.GetString("n"),
			"lost update: transactions interleaved")
	})

	t.Run("ErrorPropagates", func(t *testing.T) {
		s := f(t)
		sentinel := errors.New("boom")
		err := s.Tx(ctx(), func(store.Store) error { return sentinel })
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("NestedTxJoins", func(t *testing.T) {
		s := f(t)
		err := s.Tx(ctx(), func(tx store.Store) error {
			if err := tx.CreateEntity(ctx(), entity.New("FEAT-100", "feature")); err != nil {
				return err
			}
			return tx.Tx(ctx(), func(inner store.Store) error {
				return inner.CreateEntity(ctx(), entity.New("FEAT-101", "feature"))
			})
		})
		require.NoError(t, err)

		_, err = s.GetEntity(ctx(), "FEAT-100")
		require.NoError(t, err)
		_, err = s.GetEntity(ctx(), "FEAT-101")
		require.NoError(t, err)
	})
}

// RunTxRollbackTests exercises the stronger Tx guarantees of backends
// with real rollback (pgstore): an error from fn discards the
// transaction's writes, delivers no events, and post-commit delivery
// works. fsstore/memstore deliberately do NOT meet these (DEC-8UIL0
// reduced single-user guarantees) and must not run this suite.
func RunTxRollbackTests(t *testing.T, f Factory) {
	sentinel := errors.New("boom")

	t.Run("CreateRolledBack", func(t *testing.T) {
		s := f(t)
		err := s.Tx(ctx(), func(tx store.Store) error {
			if err := tx.CreateEntity(ctx(), entity.New("FEAT-900", "feature")); err != nil {
				return err
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)

		_, err = s.GetEntity(ctx(), "FEAT-900")
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("UpdateRolledBack", func(t *testing.T) {
		s := f(t)
		seed := entity.New("FEAT-001", "feature")
		seed.SetString("title", "Original")
		require.NoError(t, s.CreateEntity(ctx(), seed))

		err := s.Tx(ctx(), func(tx store.Store) error {
			e, err := tx.GetEntity(ctx(), "FEAT-001")
			if err != nil {
				return err
			}
			e.SetString("title", "Changed")
			if err := tx.UpdateEntity(ctx(), e); err != nil {
				return err
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)

		got, err := s.GetEntity(ctx(), "FEAT-001")
		require.NoError(t, err)
		require.Equal(t, "Original", got.GetString("title"))
	})

	t.Run("NoEventsOnRollback", func(t *testing.T) {
		s := f(t)
		events, cancel := s.Subscribe(8)
		defer cancel()

		err := s.Tx(ctx(), func(tx store.Store) error {
			if err := tx.CreateEntity(ctx(), entity.New("FEAT-900", "feature")); err != nil {
				return err
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)

		// Events are delivered synchronously before Tx returns, so an
		// empty channel here is deterministic, not a timing bet.
		select {
		case ev := <-events:
			t.Fatalf("event %v delivered for a rolled-back transaction", ev)
		default:
		}
	})

	t.Run("EventsDeliveredAfterCommit", func(t *testing.T) {
		s := f(t)
		events, cancel := s.Subscribe(8)
		defer cancel()

		err := s.Tx(ctx(), func(tx store.Store) error {
			return tx.CreateEntity(ctx(), entity.New("FEAT-100", "feature"))
		})
		require.NoError(t, err)

		select {
		case ev := <-events:
			require.Equal(t, store.EventEntityCreated, ev.Op)
			require.Equal(t, "FEAT-100", ev.EntityID)
		default:
			t.Fatal("no event delivered after committed transaction")
		}
	})
}

// TxDurabilityFactory builds a store for the abnormal-exit cases below. It is
// separate from Factory because these cases need a store that OUTLIVES the
// transaction under test — a factory that registers t.Cleanup teardown is fine,
// but the store must still be usable after fn has panicked.
//
// The cases exist because RunTxRollbackTests only ever returns an ERROR from
// fn. A backend can pass it while leaving the store permanently unusable after
// a panic or a cancelled commit, which is exactly what happened to sqlitestore:
// abandoning an open transaction on a pooled connection poisoned every
// subsequent transaction AND made the uncommitted write durable. The suite
// could not see it, so the bug shipped to review.
func RunTxAbnormalExitTests(t *testing.T, f Factory) {
	t.Run("PanicInFnLeavesStoreUsable", func(t *testing.T) {
		s := f(t)

		func() {
			defer func() {
				if recover() == nil {
					t.Error("expected the panic to propagate out of Tx")
				}
			}()
			_ = s.Tx(ctx(), func(tx store.Store) error {
				require.NoError(t, tx.CreateEntity(ctx(), entity.New("PANIC-1", "feature")))
				panic("boom")
			})
		}()

		// The write must not have survived...
		_, err := s.GetEntity(ctx(), "PANIC-1")
		require.ErrorIs(t, err, store.ErrNotFound,
			"a panicking transaction must not commit its writes")

		// ...and the store must still work. A backend that pins a connection
		// for the transaction can return it to the pool mid-transaction here,
		// poisoning every later write.
		require.NoError(t, s.Tx(ctx(), func(tx store.Store) error {
			return tx.CreateEntity(ctx(), entity.New("AFTER-PANIC", "feature"))
		}), "the store must remain usable after a panic inside Tx")

		got, err := s.GetEntity(ctx(), "AFTER-PANIC")
		require.NoError(t, err)
		require.Equal(t, "AFTER-PANIC", got.ID)
	})

	t.Run("ContextCancelledDuringTxLeavesStoreUsable", func(t *testing.T) {
		s := f(t)

		cancelCtx, cancel := context.WithCancel(context.Background())
		err := s.Tx(cancelCtx, func(tx store.Store) error {
			if err := tx.CreateEntity(cancelCtx, entity.New("CANCEL-1", "feature")); err != nil {
				return err
			}
			// Cancel between the last write and the commit — a closed browser
			// tab mid-save. The commit must not leave the transaction open.
			cancel()
			return nil
		})
		// Either outcome is acceptable: the backend may commit (the work was
		// done) or fail. What is NOT acceptable is being unusable afterwards.
		_ = err

		require.NoError(t, s.Tx(context.Background(), func(tx store.Store) error {
			return tx.CreateEntity(context.Background(), entity.New("AFTER-CANCEL", "feature"))
		}), "the store must remain usable after a cancelled Tx")
	})
}

// RunTxObserverIsolationTests asserts observers do not witness rolled-back
// writes.
//
// Separate from RunTxRollbackTests because it needs a store with an observer
// attached, which Factory cannot express. Worth its own entry point: observers
// are how derived state (search indexes) stays correct, so an observer firing
// for a write that never committed leaves the index holding a phantom entity —
// and nothing self-heals until a full reindex.
func RunTxObserverIsolationTests(t *testing.T, newStore func(t *testing.T, o store.EntityObserver) store.Store) {
	t.Run("RollbackDoesNotNotifyObservers", func(t *testing.T) {
		obs := &recordingObserver{}
		s := newStore(t, obs)

		sentinel := errors.New("boom")
		err := s.Tx(ctx(), func(tx store.Store) error {
			if err := tx.CreateEntity(ctx(), entity.New("OBS-1", "feature")); err != nil {
				return err
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)
		require.Empty(t, obs.puts,
			"an observer must not see a write that was rolled back; a search "+
				"index would hold an entity the store does not have")
	})

	t.Run("CommitNotifiesObservers", func(t *testing.T) {
		obs := &recordingObserver{}
		s := newStore(t, obs)

		require.NoError(t, s.Tx(ctx(), func(tx store.Store) error {
			return tx.CreateEntity(ctx(), entity.New("OBS-2", "feature"))
		}))
		require.Equal(t, []string{"OBS-2"}, obs.puts,
			"deferring observer callbacks must not drop them")
	})
}

// recordingObserver records the ids it was told about.
type recordingObserver struct {
	mu      sync.Mutex
	puts    []string
	deletes []string
}

func (o *recordingObserver) EntityPut(e *entity.Entity) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.puts = append(o.puts, e.ID)
	return nil
}

func (o *recordingObserver) EntityDelete(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.deletes = append(o.deletes, id)
	return nil
}

func (o *recordingObserver) EntityRenamed(_ string, renamed *entity.Entity) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.puts = append(o.puts, renamed.ID)
	return nil
}
