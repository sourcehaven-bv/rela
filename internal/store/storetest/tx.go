package storetest

import (
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
