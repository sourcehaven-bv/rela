//go:build postgres

package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestRollupLeafScanTimesOut pins that a graph query with a roll-up leaf
// runs under [pgstore.RollupStatementTimeout]: the keyset scan, the
// one-statement page, CountMatched and GraphCount are each cancelled by
// the timeout and fail closed, with an error and no rows. The timeout is
// shrunk to 1ms over a padded fixture, so the walk cannot finish in time.
func TestRollupLeafScanTimesOut(t *testing.T) {
	pool := newScopedPool(t)
	seedAlongFixture(t, pool, 2000)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	pgstore.SetRollupStatementTimeoutForTest(t, time.Millisecond)
	ctx := context.Background()

	paged := leafListQuery(true)
	keyset := paged
	keyset.Limit = 0
	for name, q := range map[string]store.GraphQuery{"page": paged, "keyset": keyset} {
		var rows int
		var scanErr error
		for _, err := range s.GraphQueryHeaders(ctx, q) {
			if err != nil {
				scanErr = err
				break
			}
			rows++
		}
		requireStatementTimeout(t, name+" headers", scanErr)
		require.Zero(t, rows, "%s headers yielded rows before the timeout", name)
		for _, err := range s.GraphQuery(ctx, q) {
			scanErr = err
			break
		}
		requireStatementTimeout(t, name+" entities", scanErr)
	}
	_, err = s.CountMatched(ctx, keyset)
	requireStatementTimeout(t, "CountMatched", err)
	_, _, err = s.GraphCount(ctx, keyset)
	requireStatementTimeout(t, "GraphCount", err)

	var after string
	require.NoError(t, pool.QueryRow(ctx, "SELECT current_setting('statement_timeout')").Scan(&after))
	require.Equal(t, "0", after, "the leaf timeout leaked onto the connection")
}

// requireStatementTimeout asserts err is PostgreSQL's statement timeout.
func requireStatementTimeout(t *testing.T, what string, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.ErrorAsf(t, err, &pgErr, "%s: want a statement timeout", what)
	require.Equalf(t, "57014", pgErr.Code, "%s: err = %v, want a statement timeout", what, err)
}
