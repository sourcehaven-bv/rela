//go:build sqlite

package appbuild_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit/sqlaudit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestSQLiteAuditGoesToTheDatabase pins the wiring of TKT-USQBMG6: on the
// sqlite build the entity manager's audit sink is the database log, and a
// write lands there as a verifiable chain row.
func TestSQLiteAuditGoesToTheDatabase(t *testing.T) {
	svc, err := discover(t, writeMinimalProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	log := svc.AuditLog()
	require.NotNil(t, log, "the sqlite build keeps its audit log in the database")

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	e := entity.New("", "doc")
	e.SetString("title", "audited")
	_, err = svc.EntityManager().CreateEntity(ctx, e, entity.CreateOptions{})
	require.NoError(t, err)

	requireRows(t, log, 1)
}

// TestSQLiteAuditSurvivesReassembly: a schema reload re-assembles over the
// same store. The successor must keep appending to the database log, and
// closing the predecessor's assembly must not close the sink the successor
// shares.
func TestSQLiteAuditSurvivesReassembly(t *testing.T) {
	origin, err := discover(t, writeMinimalProject(t))
	require.NoError(t, err)
	log := origin.AuditLog()
	require.NotNil(t, log)

	successor, err := origin.Base().ForReassembly(origin).Assemble(
		origin.Store(), origin.Searcher(), origin.VisibleSearcher(), nil)
	require.NoError(t, err)
	origin.CloseAssembly()
	require.Same(t, origin.Audit(), successor.Audit(), "the sink is shared, not rebuilt or swapped for JSONL")

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	e := entity.New("", "doc")
	e.SetString("title", "after reload")
	_, err = successor.EntityManager().CreateEntity(ctx, e, entity.CreateOptions{})
	require.NoError(t, err)
	requireRows(t, log, 1)
	require.NoError(t, successor.Close())
}

// requireRows waits for the sink to append rows rows, and checks the chain.
func requireRows(t *testing.T, log *sqlaudit.Log, rows int64) {
	t.Helper()
	var v sqlaudit.Verification
	require.Eventually(t, func() bool {
		var err error
		v, err = log.Verify(context.Background())
		return err == nil && v.Rows >= rows
	}, 5*time.Second, 10*time.Millisecond, "the audit rows never reached the database")
	require.Nil(t, v.Break)
	require.Equal(t, rows, v.Rows)
}
