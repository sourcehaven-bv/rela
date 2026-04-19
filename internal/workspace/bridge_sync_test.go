package workspace_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// bridgePaths builds a SafeFS(OsFS) + project.Context seeded with a
// valid metamodel at the standard location. Uses t.TempDir so the
// production FS stack is exercised end-to-end (app.FSFactory requires
// *storage.SafeFS to install the post-write hook).
func bridgePaths(t *testing.T) (*storage.SafeFS, *project.Context) {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	root := t.TempDir()
	paths := &project.Context{
		Root:          root,
		MetamodelPath: filepath.Join(root, "metamodel.yaml"),
		EntitiesDir:   filepath.Join(root, "entities"),
		RelationsDir:  filepath.Join(root, "relations"),
		CacheDir:      filepath.Join(root, ".rela"),
	}
	require.NoError(t, fs.MkdirAll(paths.EntitiesDir, 0o755))
	require.NoError(t, fs.MkdirAll(paths.RelationsDir, 0o755))
	require.NoError(t, fs.MkdirAll(paths.CacheDir, 0o755))
	require.NoError(t, fs.WriteFile(paths.MetamodelPath,
		[]byte(testutil.WorkspaceMetamodelYAML()), 0o644))
	return fs, paths
}

// TestFactoryInitialLoad asserts workspace.New builds the initial graph
// via the store.Factory when one is configured.
func TestFactoryInitialLoad(t *testing.T) {
	fs, paths := bridgePaths(t)

	meta, err := metamodel.Parse([]byte(testutil.WorkspaceMetamodelYAML()))
	require.NoError(t, err)
	factory := &app.FSFactory{FS: fs, Paths: paths}
	seed, err := factory.OpenStore(meta)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, seed.CreateEntity(ctx, &entity.Entity{
		ID:         "REQ-1",
		Type:       "requirement",
		Properties: map[string]interface{}{"title": "Seeded"},
	}))
	require.NoError(t, seed.CreateEntity(ctx, &entity.Entity{
		ID:         "REQ-2",
		Type:       "requirement",
		Properties: map[string]interface{}{"title": "Seeded 2"},
	}))
	_, err = seed.CreateRelation(ctx, "REQ-1", "depends-on", "REQ-2", nil)
	require.NoError(t, err)
	require.NoError(t, seed.Close())

	ws, err := workspace.New(fs, paths, workspace.NopScriptExecutor,
		workspace.WithStoreFactory(factory))
	require.NoError(t, err)
	defer ws.Close()

	ids := make([]string, 0, 2)
	for e, err := range ws.Store().ListEntities(ctx, store.EntityQuery{}) {
		require.NoError(t, err)
		ids = append(ids, e.ID)
	}
	assert.ElementsMatch(t, []string{"REQ-1", "REQ-2"}, ids)

	rels := 0
	for _, err := range ws.Store().ListRelations(ctx, store.RelationQuery{}) {
		require.NoError(t, err)
		rels++
	}
	assert.Equal(t, 1, rels, "relation should be loaded")
}
