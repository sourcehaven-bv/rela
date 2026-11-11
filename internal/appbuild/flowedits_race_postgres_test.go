//go:build postgres

package appbuild_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// waitForWriteLockWaiter waits until a session waits for the write advisory
// lock of svc's schema (pgstore's writeAdvisoryLockKey, "RELW", in the
// two-key form keyed by the schema). It fails when op finishes first.
// Postgres does not show which goroutine a session serves, so caller is
// not checked here.
func waitForWriteLockWaiter(t *testing.T, svc *appbuild.Services, op <-chan error, _ string) {
	t.Helper()
	db := pgstore.HandleFor(svc.Store())
	require.NotNil(t, db)
	const q = `SELECT count(*) FROM pg_locks
		WHERE locktype = 'advisory' AND NOT granted AND objsubid = 2
		  AND classid = x'52454c57'::int::oid
		  AND objid = hashtext(current_schema())::oid`
	deadline := time.Now().Add(writeLockWait)
	for time.Now().Before(deadline) {
		failIfFinished(t, op)
		var n int
		require.NoError(t, db.QueryRow(context.Background(), q).Scan(&n))
		if n > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no session waited for the write advisory lock")
}
