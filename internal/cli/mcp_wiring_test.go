package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// seedProject creates a minimal valid rela project in a temp dir and
// returns its root path.
func seedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "entities"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "relations"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(`
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title:
        type: string
        required: true
relations: {}
`), 0o644))
	return root
}

func TestNewMCPServices_NoProject(t *testing.T) {
	dir := t.TempDir() // empty — no metamodel.yaml

	_, err := newMCPServices(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, relaerrors.ErrNoProject,
		"missing metamodel.yaml must surface as ErrNoProject so runMCPServer can emit the user-friendly message")
}

func TestNewMCPServices_BadMetamodel(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "entities"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "relations"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"),
		[]byte("entities: this-is-not-a-map\nrelations: {}\n"), 0o644))

	_, err := newMCPServices(root)
	require.Error(t, err)
	// Real diagnostic must propagate, not be flattened to ErrNoProject.
	assert.NotErrorIs(t, err, relaerrors.ErrNoProject,
		"metamodel parse failures must NOT be wrapped as ErrNoProject; operator needs the real error")
}

func TestNewMCPServices_Succeeds(t *testing.T) {
	root := seedProject(t)

	svc, err := newMCPServices(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	assert.NotNil(t, svc.Store())
	assert.NotNil(t, svc.Meta())
	assert.NotNil(t, svc.Tracer())
	assert.NotNil(t, svc.Searcher())
	assert.NotNil(t, svc.Validator())
	assert.NotNil(t, svc.EntityManager())
	assert.NotNil(t, svc.Config())
	assert.NotNil(t, svc.Paths())
	assert.NotNil(t, svc.LuaCache())
	assert.NotNil(t, svc.Watcher())
	// Variadic var _ = (*mcpServices)(nil) on the type already
	// pins the Services interface assertion at compile time.
	var _ relamcp.Services = svc
}

func TestNewMCPServices_WritesReachSearchIndex(t *testing.T) {
	root := seedProject(t)

	svc, err := newMCPServices(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	require.NoError(t, svc.Store().CreateEntity(ctx, &entity.Entity{
		ID:         "ITEM-1",
		Type:       "item",
		Properties: map[string]interface{}{"title": "Synchronous indexing"},
	}))

	hits := make([]string, 0, 1)
	for hit, hitErr := range svc.Searcher().Search(ctx, search.Query{Text: "Synchronous"}) {
		require.NoError(t, hitErr)
		hits = append(hits, hit.ID)
	}
	assert.Contains(t, hits, "ITEM-1", "observer wiring should make the write visible to search immediately")
}

func TestMCPServices_CloseIdempotent(t *testing.T) {
	root := seedProject(t)

	svc, err := newMCPServices(root)
	require.NoError(t, err)

	// First close releases the backend + store.
	require.NoError(t, svc.Close())
	// Second close is a no-op: backend is nil, store close is idempotent.
	require.NoError(t, svc.Close())
}

func TestBackfillBackend_NilSafe(t *testing.T) {
	assert.NoError(t, backfillBackend(context.Background(), nil, memstore.New()))
}

func TestMCPWatcher_NoOpWhenStoreLacksWatcher(t *testing.T) {
	// memstore doesn't implement storeStartStopper; the adapter's
	// Start/Stop must be safe no-ops.
	w := &mcpWatcher{store: memstore.New()}
	require.NoError(t, w.Start(func() {}))
	w.Stop()
	w.Pause()
	w.Resume()
}

func TestMCPWatcher_DelegatesToStore(t *testing.T) {
	called := struct{ start, stop int }{}
	w := &mcpWatcher{
		store: recordingStartStopper{onStart: func() { called.start++ }, onStop: func() { called.stop++ }},
	}
	require.NoError(t, w.Start(func() {}))
	w.Stop()
	assert.Equal(t, 1, called.start)
	assert.Equal(t, 1, called.stop)
}

type recordingStartStopper struct {
	store.Store
	onStart func()
	onStop  func()
}

func (r recordingStartStopper) StartWatching() error {
	if r.onStart != nil {
		r.onStart()
	}
	return nil
}

func (r recordingStartStopper) StopWatching() {
	if r.onStop != nil {
		r.onStop()
	}
}

func TestMCPWatcher_StartReturnsError(t *testing.T) {
	w := &mcpWatcher{
		store: errStartStopper{err: errors.New("boom")},
	}
	err := w.Start(func() {})
	require.Error(t, err)
}

type errStartStopper struct {
	store.Store
	err error
}

func (e errStartStopper) StartWatching() error { return e.err }
func (errStartStopper) StopWatching()          {}
