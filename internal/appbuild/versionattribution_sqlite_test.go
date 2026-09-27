//go:build sqlite

package appbuild_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestSQLiteSweptVersionCreditsThePrincipal is BUG-07DNNY end to end: a write
// through the assembled entitymanager, carrying the principal an MCP or HTTP
// request stamps, must reach history under that principal once the sweep
// captures it. The store-level contract is pinned by
// storetest.RunSweepAttributionTests; this pins the wiring that feeds it.
func TestSQLiteSweptVersionCreditsThePrincipal(t *testing.T) {
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "20ms")
	t.Setenv("RELA_VERSION_SWEEP_IDLE", "1ms")

	svc, err := discover(t, writeMinimalProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	editor := principal.Principal{User: "alice", Tool: principal.ToolMCP}
	ctx := principal.With(context.Background(), editor)
	e := entity.New("", "doc")
	e.SetString("title", "attributed")
	created, err := svc.EntityManager().CreateEntity(ctx, e, entity.CreateOptions{})
	require.NoError(t, err)
	id := created.Entity.ID

	var metas []store.VersionMeta
	require.Eventually(t, func() bool {
		metas, err = svc.Versions().ListVersions(context.Background(), id)
		return err == nil && len(metas) == 1
	}, 5*time.Second, 20*time.Millisecond, "the sweep never captured %s", id)
	require.Equal(t, editor.User, metas[0].PrincipalUser)
	require.Equal(t, editor.Tool, metas[0].PrincipalTool)
}
