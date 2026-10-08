//go:build sqlite

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// bakedOnlyProject returns a project whose schema lives only in rela.db:
// the seeded schema is stored with `rela db load` semantics and then
// removed from disk, along with the markdown data directories.
func bakedOnlyProject(t *testing.T) string {
	t.Helper()
	root := seedProject(t)
	require.NoError(t, os.Rename(filepath.Join(root, "metamodel.yaml"), filepath.Join(root, "schema.yaml")))
	for _, dir := range []string{"entities", "relations"} {
		require.NoError(t, os.RemoveAll(filepath.Join(root, dir)))
	}
	fsys := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fsys)
	require.NoError(t, err)
	_, err = appbuild.LoadProjectConfig(context.Background(), fsys, paths, root,
		appbuild.ConfigImportOptions{Audit: audit.Nop{}})
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(root, "schema.yaml")))
	return root
}

// TestMCPServices_WatchSchemaThroughLayeredLoader covers schema hot-reload for
// a project whose schema lives in rela.db. The loader is
// config.NewLayered(rootfs, configsql), and only the disk layer can watch:
// the stored copy changes only through `rela db load`, which needs the
// database's exclusive lock and so cannot run beside this process. What can
// change while the server runs is the disk layer, so the test drives that:
// a schema.yaml written beside a database-only project is picked up, and
// removing it again falls back to the stored schema.
func TestMCPServices_WatchSchemaThroughLayeredLoader(t *testing.T) {
	root := bakedOnlyProject(t)
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	_, ok := svc.Deps().Meta.GetEntityDef("item")
	require.True(t, ok, "the stored schema should be served with no file on disk")
	_, ok = svc.Deps().Meta.GetEntityDef("risk")
	require.False(t, ok, "fixture must not already define risk")

	srv, err := relamcp.NewServer(svc.Deps(), "test",
		relamcp.WithPrincipal(principal.Principal{User: "t", Tool: principal.ToolMCP}))
	require.NoError(t, err)
	svc.watchSchema(srv)
	require.NotNil(t, svc.stopSchemaWatch, "the layered loader should forward Subscribe to the disk layer")

	schemaPath := filepath.Join(root, "schema.yaml")
	require.NoError(t, os.WriteFile(schemaPath, []byte(schemaWithRisk), 0o644))
	assert.Eventually(t, func() bool {
		_, ok := svc.Deps().Meta.GetEntityDef("risk")
		return ok
	}, 5*time.Second, 50*time.Millisecond, "watcher never picked up the schema written on disk")

	require.NoError(t, os.Remove(schemaPath))
	assert.Eventually(t, func() bool {
		_, ok := svc.Deps().Meta.GetEntityDef("risk")
		return !ok
	}, 5*time.Second, 50*time.Millisecond, "removing the disk schema did not fall back to the stored one")
}
