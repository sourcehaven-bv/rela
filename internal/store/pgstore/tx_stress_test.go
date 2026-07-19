package pgstore_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// poolForSchema opens a SECOND, independent pool pinned to an
// already-migrated test schema. Separate pool = separate pg sessions, which
// is what makes the cross-store test meaningful: advisory locks are
// per-session, so two pools model two deployment processes.
func poolForSchema(t *testing.T, schema string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	cfg.MaxConns = 2
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// TestTxCrossStoreSerialization proves the DEPLOYMENT-WIDE claim of the Tx
// contract: two independent stores over two independent pools (= two pg
// sessions, modeling two rela-server processes) run concurrent Tx
// read-modify-write increments against the same schema. Only the advisory
// lock serializes them — a process-local mechanism (the writeMu bug class)
// would lose updates here almost immediately under READ COMMITTED.
func TestTxCrossStoreSerialization(t *testing.T) {
	admin := adminConn(t)
	pool1, schema, err := createScopedPool(admin)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool1.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgQuoteIdent(schema)+" CASCADE")
	})
	pool2 := poolForSchema(t, schema)

	s1, err := pgstore.New(pool1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s1.Close() })
	s2, err := pgstore.New(pool2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s2.Close() })

	ctx := context.Background()
	seed := entity.New("STRS-CTR", "feature")
	seed.SetString("n", "0")
	require.NoError(t, s1.CreateEntity(ctx, seed))

	const workers, loops = 8, 5
	stores := []store.Store{s1, s2}
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			s := stores[w%2]
			for range loops {
				if txErr := s.Tx(ctx, func(tx store.Store) error {
					e, gerr := tx.GetEntity(ctx, "STRS-CTR")
					if gerr != nil {
						return gerr
					}
					n, perr := strconv.Atoi(e.GetString("n"))
					if perr != nil {
						return perr
					}
					e.SetString("n", strconv.Itoa(n+1))
					return tx.UpdateEntity(ctx, e)
				}); txErr != nil {
					errs[w] = txErr
					return
				}
			}
		})
	}
	wg.Wait()
	for w, err := range errs {
		require.NoError(t, err, "worker %d", w)
	}

	// Read through the OTHER store to also prove cross-store visibility.
	got, err := s2.GetEntity(ctx, "STRS-CTR")
	require.NoError(t, err)
	require.Equal(t, strconv.Itoa(workers*loops), got.GetString("n"),
		"lost update across stores: advisory lock is not serializing sessions")
}

// TestTxPoolExhaustion pins the no-second-connection property: a Tx must
// never acquire another pool connection while holding one (that cycle is
// the realistic pgstore deadlock). The scoped test pool has MaxConns=2;
// 16 concurrent multi-write transactions must all complete anyway, just
// slowly. Afterwards the write advisory key must not appear in pg_locks
// and no session may sit idle-in-transaction — both would indicate a
// leaked transaction on some error path.
func TestTxPoolExhaustion(t *testing.T) {
	pool := newScopedPool(t) // MaxConns=2 (see createScopedPool)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const workers = 16
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			errs[w] = s.Tx(ctx, func(tx store.Store) error {
				for j := range 3 {
					if cerr := tx.CreateEntity(ctx, entity.New(fmt.Sprintf("STRS-X%dY%d", w, j), "feature")); cerr != nil {
						return cerr
					}
				}
				return nil
			})
		})
	}
	wg.Wait()
	for w, err := range errs {
		require.NoError(t, err, "worker %d (starved on a 2-conn pool?)", w)
	}

	n, err := s.CountEntities(ctx, store.EntityQuery{Type: "feature"})
	require.NoError(t, err)
	require.Equal(t, workers*3, n)

	var leaked int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_locks
		WHERE locktype = 'advisory'
		  AND ((classid::bigint << 32) | objid::bigint) = $1`,
		pgstore.WriteAdvisoryLockKeyForTest).Scan(&leaked))
	require.Zero(t, leaked, "write advisory lock leaked in pg_locks")

	var idleInTx int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database()
		  AND state = 'idle in transaction'
		  AND pid <> pg_backend_pid()`).Scan(&idleInTx))
	require.Zero(t, idleInTx, "session left idle-in-transaction: leaked Tx")
}

// TestTxStress runs the shared mixed-workload soak (watchdogged deadlock
// detection + lost-update and pair-atomicity invariants) against pgstore.
// Duration 2s by default; RELA_STRESS_SECONDS extends it for local shakes.
func TestTxStress(t *testing.T) {
	storetest.RunTxStressTest(t, factory)
}
