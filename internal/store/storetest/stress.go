package storetest

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stressDuration returns the soak length: 2s by default (CI-budget), or
// RELA_STRESS_SECONDS for longer local shakes.
func stressDuration() time.Duration {
	if v := os.Getenv("RELA_STRESS_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 2 * time.Second
}

// errStressRollback is the deliberate failure the pair worker injects to
// exercise the Tx error path (rollback on pg, documented no-op on fs/mem).
var errStressRollback = errors.New("storetest: deliberate stress rollback")

// RunTxStressTest soaks a store under sustained mixed concurrent load —
// Tx read-modify-write counters, multi-write transactions with injected
// failures, nested transactions, plain writes, reads, and a draining
// subscriber — with a progress watchdog that converts a deadlock into a
// failing test with a full goroutine dump (Go's runtime deadlock panic
// only fires when ALL goroutines block, which a partial deadlock never
// triggers).
//
// End-state invariants, valid for every backend regardless of rollback
// support:
//
//   - No lost updates: the counter equals the number of Tx increments
//     that returned success.
//   - Pair atomicity: each two-entity transaction either left both
//     entities or neither (the failure is injected only after both
//     writes, so this holds under rollback AND under the fs/mem
//     no-rollback contract).
//
// Duration: 2s default; set RELA_STRESS_SECONDS for longer local runs.
func RunTxStressTest(t *testing.T, f Factory) {
	s := f(t)

	seed := entity.New("STRS-CTR", "feature")
	seed.SetString("n", "0")
	require.NoError(t, s.CreateEntity(ctx(), seed))

	var (
		stop     = make(chan struct{})
		progress atomic.Int64 // bumped by every worker iteration (watchdog food)
		commits  atomic.Int64 // successful counter-increment transactions
		pairSeq  atomic.Int64 // pair transactions attempted (ids 1..pairSeq)
		wg       sync.WaitGroup
		failures = make(chan error, 32) // first unexpected worker error wins
	)
	fail := func(err error) {
		select {
		case failures <- err:
		default:
		}
	}
	stopping := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}

	// Two counter workers: serialized read-modify-write increments. Any
	// interleaving between Tx callbacks loses an update and fails the
	// end-state check.
	for range 2 {
		wg.Go(func() {
			for !stopping() {
				err := s.Tx(ctx(), func(tx store.Store) error {
					e, err := tx.GetEntity(ctx(), "STRS-CTR")
					if err != nil {
						return err
					}
					n, err := strconv.Atoi(e.GetString("n"))
					if err != nil {
						return err
					}
					runtime.Gosched()
					e.SetString("n", strconv.Itoa(n+1))
					return tx.UpdateEntity(ctx(), e)
				})
				if err != nil {
					fail(fmt.Errorf("counter tx: %w", err))
					return
				}
				commits.Add(1)
				progress.Add(1)
			}
		})
	}

	// Pair worker: two creates per Tx, failing every third transaction
	// AFTER both writes — exercises the rollback/no-rollback path while
	// keeping the both-or-neither invariant checkable on every backend.
	wg.Go(func() {
		for !stopping() {
			i := pairSeq.Add(1)
			err := s.Tx(ctx(), func(tx store.Store) error {
				if err := tx.CreateEntity(ctx(), entity.New(fmt.Sprintf("STRS-PA%d", i), "feature")); err != nil {
					return err
				}
				if err := tx.CreateEntity(ctx(), entity.New(fmt.Sprintf("STRS-PB%d", i), "feature")); err != nil {
					return err
				}
				if i%3 == 0 {
					return errStressRollback
				}
				return nil
			})
			if err != nil && !errors.Is(err, errStressRollback) {
				fail(fmt.Errorf("pair tx %d: %w", i, err))
				return
			}
			progress.Add(1)
		}
	})

	// Nested-Tx worker: create in the outer transaction, update through a
	// joined inner one.
	wg.Go(func() {
		for j := 0; !stopping(); j++ {
			id := fmt.Sprintf("STRS-N%d", j)
			err := s.Tx(ctx(), func(tx store.Store) error {
				e := entity.New(id, "feature")
				if err := tx.CreateEntity(ctx(), e); err != nil {
					return err
				}
				return tx.Tx(ctx(), func(inner store.Store) error {
					e.SetString("title", "nested")
					return inner.UpdateEntity(ctx(), e)
				})
			})
			if err != nil {
				fail(fmt.Errorf("nested tx %s: %w", id, err))
				return
			}
			progress.Add(1)
		}
	})

	// Plain writer: ordinary create/update/delete cycles OUTSIDE any Tx,
	// contending on the Tx serialization from the non-transactional side.
	wg.Go(func() {
		for j := 0; !stopping(); j++ {
			id := fmt.Sprintf("STRS-W%d", j%50)
			e := entity.New(id, "feature")
			if err := s.CreateEntity(ctx(), e); err != nil && !errors.Is(err, store.ErrConflict) {
				fail(fmt.Errorf("plain create %s: %w", id, err))
				return
			}
			e.SetString("title", "plain")
			if err := s.UpdateEntity(ctx(), e); err != nil && !errors.Is(err, store.ErrNotFound) {
				fail(fmt.Errorf("plain update %s: %w", id, err))
				return
			}
			_, derr := s.DeleteEntity(ctx(), id, false)
			if derr != nil && !errors.Is(derr, store.ErrNotFound) && !errors.Is(derr, store.ErrHasRelations) {
				fail(fmt.Errorf("plain delete %s: %w", id, derr))
				return
			}
			progress.Add(1)
		}
	})

	// Reader: never takes any write lock; must keep flowing throughout.
	wg.Go(func() {
		for !stopping() {
			if _, err := s.GetEntity(ctx(), "STRS-CTR"); err != nil {
				fail(fmt.Errorf("reader: %w", err))
				return
			}
			n := 0
			for _, err := range s.ListEntities(ctx(), store.EntityQuery{Type: "feature"}) {
				if err != nil {
					// Tolerated: fsstore loads entity files lazily during
					// iteration, so a concurrent delete between the index
					// snapshot and the file read surfaces as a not-found
					// error mid-list. Pre-existing list-vs-delete race,
					// independent of Tx. Anything else is a real failure.
					if errors.Is(err, store.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
						break
					}
					fail(fmt.Errorf("reader list: %w", err))
					return
				}
				n++
			}
			progress.Add(1)
		}
	})

	// Subscriber: drains events for the whole run (excluded from the
	// watchdog — event flow may legitimately go quiet).
	events, cancel := s.Subscribe(1024)
	defer cancel()
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			case <-events:
			}
		}
	})

	// Watchdog: if no worker makes progress for 5s — while running or
	// while shutting down — dump every goroutine and fail. This is the
	// deadlock detector.
	wedged := make(chan string, 1)
	// workersGone closes once stop is signaled AND every worker exited.
	workersGone := make(chan struct{})
	go func() {
		<-stop
		wg.Wait()
		close(workersGone)
	}()
	go func() {
		last, stale := int64(-1), 0
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-workersGone:
				return
			case <-ticker.C:
				cur := progress.Load()
				if cur == last {
					if stale++; stale >= 10 {
						buf := make([]byte, 1<<20)
						n := runtime.Stack(buf, true)
						select {
						case wedged <- string(buf[:n]):
						default:
						}
						return
					}
				} else {
					last, stale = cur, 0
				}
			}
		}
	}()

	select {
	case dump := <-wedged:
		t.Fatalf("stress: no progress for 5s (deadlock?); goroutines:\n%s", dump)
	case <-time.After(stressDuration()):
	}
	close(stop)

	select {
	case dump := <-wedged:
		t.Fatalf("stress: workers wedged during shutdown; goroutines:\n%s", dump)
	case <-time.After(30 * time.Second):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("stress: workers did not stop within 30s; goroutines:\n%s", buf[:n])
	case <-workersGone:
	}

	select {
	case err := <-failures:
		t.Fatalf("stress worker failed: %v", err)
	default:
	}

	// Invariant: no lost counter updates.
	got, err := s.GetEntity(ctx(), "STRS-CTR")
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(commits.Load(), 10), got.GetString("n"),
		"lost update: %s counter increments committed", got.GetString("n"))

	// Invariant: every pair transaction left both entities or neither.
	for i := int64(1); i <= pairSeq.Load(); i++ {
		_, errA := s.GetEntity(ctx(), fmt.Sprintf("STRS-PA%d", i))
		_, errB := s.GetEntity(ctx(), fmt.Sprintf("STRS-PB%d", i))
		require.Equal(t, errA == nil, errB == nil,
			"pair %d torn: A present=%v B present=%v", i, errA == nil, errB == nil)
	}
	t.Logf("stress: %d counter commits, %d pair txs, final entity sweep clean",
		commits.Load(), pairSeq.Load())
}
