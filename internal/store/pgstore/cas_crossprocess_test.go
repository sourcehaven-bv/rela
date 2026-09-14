//go:build postgres

package pgstore_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// This file is the acceptance evidence for TKT-34XS2R: the cross-process case
// that `writeMu` structurally cannot address.
//
// The shape mirrors internal/dataentry/webhook_conflict_postgres_test.go
// (TestWebhookConflict_CrossProcessAppendsCanBeLost), which demonstrates an
// append being SILENTLY LOST across two pgstore handles on one schema. Two
// separate pools + two separate pgstore.New handles is the closest in-process
// analogue of two rela-server processes: they share only the database, exactly
// as `docs/postgres-backend.md` describes, and no Go-level mutex is common to
// both. Anything a process-local lock could fix is therefore excluded by
// construction.

// twoHandlesOneSchema creates one migrated schema and returns two INDEPENDENT
// stores over it, each on its own pool — the stand-in for two server
// processes. The DSN is schema-pinned via search_path rather than bare
// (CLAUDE.md): a bare DSN resolves to `public`, which is precisely the one
// configuration where schema-scoping bugs cannot show up.
func twoHandlesOneSchema(t *testing.T) (a, b store.Store) {
	t.Helper()
	admin := adminConn(t)

	pool1, schema, err := createScopedPool(admin)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool1.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgQuoteIdent(schema)+" CASCADE")
	})

	// Second pool onto the SAME schema, built independently.
	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	cfg.MaxConns = 2
	cfg.MinConns = 0
	pool2, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool2.Close)

	s1, err := pgstore.New(pool1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s1.Close() })

	s2, err := pgstore.New(pool2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s2.Close() })

	return s1, s2
}

// TestCrossProcess_UnconditionalUpdateSilentlyLosesAnAppend documents the
// PRE-CAS behavior, and is the reason this ticket exists. It is not a
// regression guard on a bug — it pins a real property of UpdateEntity, which
// is last-write-wins by contract. Its value is as the control for the test
// below: without it, "the CAS test passes" would not tell you the hazard was
// ever real on this backend.
func TestCrossProcess_UnconditionalUpdateSilentlyLosesAnAppend(t *testing.T) {
	a, b := twoHandlesOneSchema(t)
	ctx := context.Background()

	base := entity.New("FEAT-001", "feature")
	base.SetString("title", "Log")
	base.Content = "start"
	require.NoError(t, a.CreateEntity(ctx, base))

	// Both handles read the SAME base state, as two processes would.
	readA, err := a.GetEntity(ctx, base.ID)
	require.NoError(t, err)
	readB, err := b.GetEntity(ctx, base.ID)
	require.NoError(t, err)

	readA.Content += "[A]"
	require.NoError(t, a.UpdateEntity(ctx, readA))

	readB.Content += "[B]"
	require.NoError(t, b.UpdateEntity(ctx, readB), "the loser is not even told it lost")

	got, err := a.GetEntity(ctx, base.ID)
	require.NoError(t, err)
	assert.NotContains(t, got.Content, "[A]",
		"A's append is silently discarded — this is the hazard TKT-34XS2R fixes")
	assert.Contains(t, got.Content, "[B]")
}

// TestCrossProcess_ConditionalUpdateConflictsInsteadOfLosing is the
// acceptance criterion: the same interleaving, with UpdateEntityIf, must
// REPORT the conflict rather than overwrite. No process-local lock is
// involved — the two stores share nothing but the database.
func TestCrossProcess_ConditionalUpdateConflictsInsteadOfLosing(t *testing.T) {
	a, b := twoHandlesOneSchema(t)
	ctx := context.Background()

	base := entity.New("FEAT-001", "feature")
	base.SetString("title", "Log")
	base.Content = "start"
	require.NoError(t, a.CreateEntity(ctx, base))

	readA, err := a.GetEntity(ctx, base.ID)
	require.NoError(t, err)
	versionA := store.VersionOf(readA)
	readB, err := b.GetEntity(ctx, base.ID)
	require.NoError(t, err)
	versionB := store.VersionOf(readB)
	require.Equal(t, versionA, versionB,
		"two handles reading identical stored state must derive the same token")

	readA.Content += "[A]"
	_, err = a.UpdateEntityIf(ctx, readA, store.UpdateCondition{ExpectedVersion: versionA})
	require.NoError(t, err, "the first writer wins outright")

	readB.Content += "[B]"
	_, err = b.UpdateEntityIf(ctx, readB, store.UpdateCondition{ExpectedVersion: versionB})
	require.Error(t, err, "the second writer MUST be told it lost, not silently win")

	var conflict *store.VersionConflictError
	require.ErrorAs(t, err, &conflict,
		"the conflict must survive as a typed error across the store boundary (RR-HI9QIU)")
	assert.Equal(t, base.ID, conflict.ID)
	assert.Equal(t, versionB, conflict.Expected)

	got, err := b.GetEntity(ctx, base.ID)
	require.NoError(t, err)
	assert.Contains(t, got.Content, "[A]", "the winner's write survives")
	assert.NotContains(t, got.Content, "[B]", "the rejected write applied nothing")
	assert.Equal(t, store.VersionOf(got), conflict.Actual,
		"Actual must let the loser retry without guessing")
}

// TestCrossProcess_ConcurrentAppendersAllLandAcrossHandles is the end state
// the webhook append path wants: N appenders spread over two independent
// store handles, each retrying on conflict, and every append present at the
// end. This is the same workload that loses writes under UpdateEntity.
func TestCrossProcess_ConcurrentAppendersAllLandAcrossHandles(t *testing.T) {
	a, b := twoHandlesOneSchema(t)
	ctx := context.Background()

	base := entity.New("FEAT-001", "feature")
	base.SetString("title", "Log")
	base.Content = ""
	require.NoError(t, a.CreateEntity(ctx, base))

	const appenders = 8
	const maxAttempts = 200
	handles := []store.Store{a, b}

	var wg sync.WaitGroup
	errs := make([]error, appenders)
	for i := range appenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Alternate handles so half the contention is genuinely
			// cross-handle rather than within one store's connection pool.
			h := handles[i%len(handles)]
			marker := fmt.Sprintf("[line-%d]", i)
			for range maxAttempts {
				cur, err := h.GetEntity(ctx, base.ID)
				if err != nil {
					errs[i] = err
					return
				}
				v := store.VersionOf(cur)
				cur.Content += marker
				if _, err = h.UpdateEntityIf(ctx, cur, store.UpdateCondition{ExpectedVersion: v}); err == nil {
					return
				}
				var conflict *store.VersionConflictError
				if !errors.As(err, &conflict) {
					errs[i] = err
					return
				}
			}
			errs[i] = fmt.Errorf("appender %d exhausted %d attempts", i, maxAttempts)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		require.NoErrorf(t, err, "appender %d", i)
	}

	got, err := a.GetEntity(ctx, base.ID)
	require.NoError(t, err)
	for i := range appenders {
		assert.Containsf(t, got.Content, fmt.Sprintf("[line-%d]", i),
			"appender %d's write was lost across handles despite CAS", i)
	}
}
