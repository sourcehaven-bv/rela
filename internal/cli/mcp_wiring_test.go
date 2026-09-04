package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/principal"
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

// mcpServicesForTest is newMCPServices with a backend the current build can
// actually open.
//
// newMCPServices calls appbuild.Discover, which resolves its DSN from the
// process environment, so unlike the appbuild tests this cannot pass an option
// — backendtest.Env supplies the variables instead and t.Setenv unwinds them.
// On fs/memory the map is empty and this is exactly newMCPServices.
//
// The tests below assert MCP wiring (deps populated, writes reach the search
// index, Close is idempotent), none of which is backend-specific; they simply
// need a store to exist.
func mcpServicesForTest(t *testing.T, root string) (*mcpServices, error) {
	t.Helper()
	for k, v := range backendtest.Env(t) {
		t.Setenv(k, v)
	}
	return newMCPServices(root)
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

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	deps := svc.Deps()
	assert.NotNil(t, deps.Store)
	assert.NotNil(t, deps.Meta)
	assert.NotNil(t, deps.Tracer)
	assert.NotNil(t, deps.Searcher)
	assert.NotNil(t, deps.Validator)
	assert.NotNil(t, deps.EntityManager)
	assert.NotNil(t, deps.Config)
	assert.NotNil(t, deps.LuaCache)
	assert.NotNil(t, deps.Watcher)
	assert.NotEmpty(t, deps.ProjectRoot)
}

func TestNewMCPServices_WritesReachSearchIndex(t *testing.T) {
	root := seedProject(t)

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	deps := svc.Deps()
	require.NoError(t, svc.svc.Store().CreateEntity(ctx, &entity.Entity{
		ID:         "ITEM-1",
		Type:       "item",
		Properties: map[string]any{"title": "Synchronous indexing"},
	}))

	hits := make([]string, 0, 1)
	for hit, hitErr := range deps.Searcher.Search(ctx, search.Query{Text: "Synchronous"}) {
		require.NoError(t, hitErr)
		hits = append(hits, hit.ID)
	}
	assert.Contains(t, hits, "ITEM-1", "observer wiring should make the write visible to search immediately")
}

func TestMCPServices_CloseIdempotent(t *testing.T) {
	root := seedProject(t)

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)

	// First close releases the backend + store.
	require.NoError(t, svc.Close())
	// Second close is a no-op: backend is nil, store close is idempotent.
	require.NoError(t, svc.Close())
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

// writeSchema overwrites the project's schema file with body.
func writeSchema(t *testing.T, root, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(body), 0o644))
}

// schemaWithRisk is the seeded schema plus a second entity type, standing in
// for an operator adding a type mid-session.
const schemaWithRisk = `
entities:
  item:
    label: Item
    id_type: sequential
    id_prefix: ITEM-
    properties:
      title:
        type: string
        required: true
  risk:
    label: Risk
    id_type: sequential
    id_prefix: RSK-
    properties:
      title:
        type: string
        required: true
relations: {}
`

// TestMCPServices_ReloadPicksUpNewType is the end-to-end wiring property of
// TKT-NU247U: editing the schema on disk and reloading yields deps whose
// metamodel — and whose entitymanager — know the new type.
func TestMCPServices_ReloadPicksUpNewType(t *testing.T) {
	root := seedProject(t)

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	_, hadRisk := svc.Deps().Meta.GetEntityDef("risk")
	require.False(t, hadRisk, "fixture must not already define risk")

	writeSchema(t, root, schemaWithRisk)

	deps, err := svc.reload()
	require.NoError(t, err)

	_, ok := deps.Meta.GetEntityDef("risk")
	assert.True(t, ok, "reloaded metamodel should define the new type")

	// The write path must move too — it holds its own metamodel, so a reload
	// that refreshed only Deps.Meta would leave creates failing on the new type.
	_, err = deps.EntityManager.CreateEntity(context.Background(),
		&entity.Entity{Type: "risk", Properties: map[string]any{"title": "Supplier outage"}},
		entity.CreateOptions{})
	assert.NoError(t, err, "entitymanager should accept the newly declared type after reload")
}

// TestMCPServices_ReloadReusesStoreAndSearcher pins the reason reload does not
// simply re-run Discover: the store holds the data and the searcher its index,
// and a schema edit invalidates neither. Reopening them would mean a full
// reindex on every save — and would drop entities written this session.
func TestMCPServices_ReloadReusesStoreAndSearcher(t *testing.T) {
	root := seedProject(t)

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	ctx := context.Background()
	before := svc.svc.Store()
	require.NoError(t, before.CreateEntity(ctx, &entity.Entity{
		ID: "ITEM-1", Type: "item", Properties: map[string]any{"title": "Written before reload"},
	}))

	writeSchema(t, root, schemaWithRisk)
	deps, err := svc.reload()
	require.NoError(t, err)

	assert.Same(t, before, svc.svc.Store(), "reload must reuse the open store, not reopen it")

	// The store stays usable and keeps this session's writes.
	got, err := svc.svc.Store().GetEntity(ctx, "ITEM-1")
	require.NoError(t, err, "store unusable after reload")
	assert.Equal(t, "ITEM-1", got.ID)

	// And the search index is the same one, still holding the pre-reload write.
	hits := make([]string, 0, 1)
	for hit, hitErr := range deps.Searcher.Search(ctx, search.Query{Text: "Written"}) {
		require.NoError(t, hitErr)
		hits = append(hits, hit.ID)
	}
	assert.Contains(t, hits, "ITEM-1", "reload must not discard the search index")
}

// TestMCPServices_ReloadBadSchemaKeepsPrevious pins the fail-safe direction. A
// watcher fires on every save, and a save taken mid-edit is routinely
// unparseable; that must not take the session's schema down.
func TestMCPServices_ReloadBadSchemaKeepsPrevious(t *testing.T) {
	root := seedProject(t)

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	good := svc.svc

	writeSchema(t, root, "entities: this-is-not-a-map\nrelations: {}\n")

	_, err = svc.reload()
	require.Error(t, err, "an unparseable schema must be reported, not adopted")

	assert.Same(t, good, svc.svc, "a failed reload must leave the previous services in place")
	_, ok := svc.Deps().Meta.GetEntityDef("item")
	assert.True(t, ok, "previous metamodel must still be serving after a failed reload")
}

// TestMCPServices_CloseAfterReloadIsClean covers the shutdown sequence a
// reloaded process actually runs: the current assembly and the one owning the
// store are different objects, and both must be torn down exactly once.
func TestMCPServices_CloseAfterReloadIsClean(t *testing.T) {
	root := seedProject(t)

	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)

	writeSchema(t, root, schemaWithRisk)
	_, err = svc.reload()
	require.NoError(t, err)

	require.NotSame(t, svc.origin, svc.svc, "reload should have produced a successor assembly")

	require.NoError(t, svc.Close())
	require.NoError(t, svc.Close(), "Close must stay idempotent after a reload")
}

// TestMCPServices_WatchSchemaFiresOnRealEdit drives the watcher end to end:
// a real write to schema.yaml on disk must reach the published deps without
// anyone calling reload() directly.
func TestMCPServices_WatchSchemaFiresOnRealEdit(t *testing.T) {
	root := seedProject(t)
	svc, err := mcpServicesForTest(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	srv, err := relamcp.NewServer(svc.Deps(), "test",
		relamcp.WithPrincipal(principal.Principal{User: "t", Tool: principal.ToolMCP}))
	require.NoError(t, err)

	svc.watchSchema(srv)
	require.NotNil(t, svc.stopSchemaWatch, "watcher should have started")

	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(schemaWithRisk), 0o644))

	assert.Eventually(t, func() bool {
		_, ok := svc.Deps().Meta.GetEntityDef("risk")
		return ok
	}, 5*time.Second, 50*time.Millisecond, "watcher never picked up the schema edit")
}
