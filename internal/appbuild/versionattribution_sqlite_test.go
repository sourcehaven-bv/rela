//go:build sqlite

package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
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
		metas, err = svc.Versions().ListVersions(context.Background(), entity.Ref{ID: id})
		return err == nil && len(metas) == 1
	}, 5*time.Second, 20*time.Millisecond, "the sweep never captured %s", id)
	require.Equal(t, editor.User, metas[0].PrincipalUser)
	require.Equal(t, editor.Tool, metas[0].PrincipalTool)
}

// copyProjectYAML declares a cross-entity copy from a faced page to a faceless
// note, the shape whose source is a different row.
const copyProjectYAML = `version: "1.0"
entities:
  page:
    label: Page
    id_prefix: "PAGE-"
    id_type: sequential
    faces:
      live: {}
    properties:
      title: {type: string}
  note:
    label: Note
    id_prefix: "NOTE-"
    id_type: sequential
    properties:
      title: {type: string}
copies:
  note-from-page:
    from: page@live
    to: note
    fields:
      title: "{{new.title}}"
`

// TestSQLiteSweptCopyNamesItsSource is BUG-YC5Z07 end to end: a copy through
// the assembled entitymanager must reach sqlite history marked as a copy of
// its source once the sweep captures it. The store-level contract is pinned by
// storetest.RunSweepOriginTests; this pins the wiring that feeds it.
func TestSQLiteSweptCopyNamesItsSource(t *testing.T) {
	t.Setenv("RELA_VERSION_SWEEP_INTERVAL", "20ms")
	t.Setenv("RELA_VERSION_SWEEP_IDLE", "1ms")

	root := writeMinimalProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(copyProjectYAML), 0o644))
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	page := entity.New("", "page")
	page.SetString("title", "published")
	created, err := svc.EntityManager().CreateEntity(ctx, page, entity.CreateOptions{Face: "live"})
	require.NoError(t, err)
	source := created.Entity.ID
	_, err = svc.EntityManager().CopyState(ctx, entitymanager.CopyRequest{
		Definition: "note-from-page", SourceID: source, TargetID: "NOTE-1",
	})
	require.NoError(t, err)

	var metas []store.VersionMeta
	require.Eventually(t, func() bool {
		metas, err = svc.Versions().ListVersions(context.Background(), entity.Ref{ID: "NOTE-1"})
		return err == nil && len(metas) == 1
	}, 5*time.Second, 20*time.Millisecond, "the sweep never captured NOTE-1")
	require.Equal(t, store.OriginCopy, metas[0].Origin.Kind)
	require.Equal(t, "note-from-page", metas[0].Origin.Definition)
	require.Equal(t, source+"@live", metas[0].Origin.SourceLabel())
	require.Equal(t, "alice", metas[0].PrincipalUser)
}
