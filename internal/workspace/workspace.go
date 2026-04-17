// Package workspace provides a stateful domain session that owns the
// repository, graph, metamodel, and automation engine. It provides
// write-through operations that keep disk and in-memory state in sync,
// eliminating the dual-write pattern that consumers would otherwise
// duplicate.
package workspace

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"context"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// ChangeEvent is re-exported from storage so consumers don't need to
// import storage directly for watcher callback signatures.
type ChangeEvent = storage.ChangeEvent

// ChangeOp is re-exported from storage for the same reason as ChangeEvent.
type ChangeOp = storage.ChangeOp

// ScriptExecutor runs scripts with entity context. This follows dependency
// inversion: workspace defines the interface it needs, script package implements it.
// This keeps workspace independent of specific script languages (Lua, etc.).
//
// The executor is stateless - all context is passed at execution time via
// metamodel.ScriptContext. This avoids circular dependencies: workspace can be
// created with a script engine, and the engine receives workspace access only
// when executing scripts.
//
// For production, pass script.NewEngine(). For tests, pass NopScriptExecutor.
type ScriptExecutor interface {
	// ExecuteCode runs inline script code with entity context.
	ExecuteCode(code string, ctx metamodel.ScriptContext) error
	// ExecuteFile runs a script file from the scripts/ directory.
	ExecuteFile(path string, ctx metamodel.ScriptContext) error
}

// scriptContextImpl implements metamodel.ScriptContext for passing to ScriptExecutor.
// GetWorkspace() returns a lua.Services (via Workspace.LuaServices); the
// script package type-asserts to that before running.
type scriptContextImpl struct {
	workspace   *Workspace
	meta        *metamodel.Metamodel
	projectRoot string
	entity      *entity.Entity
	oldEntity   *entity.Entity
}

func (c *scriptContextImpl) GetWorkspace() interface{}     { return c.workspace.luaServices() }
func (c *scriptContextImpl) GetMeta() *metamodel.Metamodel { return c.meta }
func (c *scriptContextImpl) GetProjectRoot() string        { return c.projectRoot }
func (c *scriptContextImpl) GetEntity() *entity.Entity     { return c.entity }
func (c *scriptContextImpl) GetOldEntity() *entity.Entity  { return c.oldEntity }

// NopScriptExecutor is a no-op implementation of ScriptExecutor for tests
// that don't trigger Lua automations. It panics if actually called, making
// it obvious when a test unexpectedly triggers Lua execution.
var NopScriptExecutor ScriptExecutor = nopScriptExecutor{}

type nopScriptExecutor struct{}

func (nopScriptExecutor) ExecuteCode(string, metamodel.ScriptContext) error {
	panic("NopScriptExecutor: Lua execution not expected in this context")
}

func (nopScriptExecutor) ExecuteFile(string, metamodel.ScriptContext) error {
	panic("NopScriptExecutor: Lua execution not expected in this context")
}

// workspaceState holds the reloadable parts of a Workspace as an immutable
// snapshot. A reload publishes a new state via atomic.Pointer.Store(); readers
// call Load() once and work against the resulting snapshot, guaranteeing
// that graph, meta, automation, and searchIdx are always observed as a
// coherent tuple from the same reload epoch.
//
// Reload publishes a fresh workspaceState — readers holding a pre-reload
// snapshot continue to see a fully-populated, never-mutated-by-Reload
// world.
type workspaceState struct {
	store      store.Store
	meta       *metamodel.Metamodel
	automation *automation.Engine // may be nil if the metamodel has no automations
	searchIdx  *search.Index      // may be nil if index construction failed
}

// Workspace is a stateful domain session that ties together the repository
// (persistence), graph (in-memory query), metamodel (schema), and automation
// engine. All write operations go through Workspace so that disk and memory
// stay in sync.
//
// # Concurrency model
//
// All reloadable state (graph, metamodel, automation engine, search index)
// is held in an immutable workspaceState struct and published via
// atomic.Pointer. Readers call state.Load() once and work against a
// coherent snapshot — no torn reads, no out-of-order publication, no need
// to coordinate two atomic loads.
//
// Reloads are serialized against each other and against Close via reloadMu;
// they build a new state (with a freshly-synced graph from disk) and
// publish it atomically. Readers holding a pre-reload state keep their
// graph reference forever — it is never mutated by Reload.
//
// Mutations (CreateEntity, UpdateEntity, DeleteEntity, CreateRelation,
// UpdateRelation, DeleteRelation) modify the currently-published graph
// in place. The caller is expected to serialize mutations against each
// other and against Reload — within the data-entry server this happens
// via App.writeMu. Each mutation method captures a single state snapshot
// at entry and uses that snapshot consistently throughout, so a Reload
// that arrives mid-mutation cannot interleave on the wrong graph.
type Workspace struct {
	fs         storage.FS
	paths      *project.Context
	store      store.Store // optional; nil when not wired
	state      atomic.Pointer[workspaceState]
	config     *project.Config
	scriptExec ScriptExecutor

	// reloadMu serializes Reload/Sync/Close against each other.
	// Readers never acquire it — they snapshot state via state.Load().
	reloadMu sync.Mutex

	// closed is set once Close has been called. Reload becomes a no-op
	// after the workspace is closed.
	closed atomic.Bool

	// Watcher state (nil when not watching).
	watcher         *storage.Watcher
	stopSchema      func() // stops the metamodel loader subscription
	stopStoreEvents func() // cancels the store event subscription

	// storeFactory, when non-nil, is consulted by Sync/Reload to
	// (re)build the graph from a store.Store instead of from repo.Sync.
	// This is the bridge used during the migration away from the
	// repository + graph duo toward store.Store-based reads.
	storeFactory store.Factory
	syncStore    store.Store // lazily opened on first Sync/Reload

	// schemaFiles holds the absolute paths of all files that make up the
	// current metamodel (metamodel.yaml + includes). Updated after each
	// successful Reload.
	schemaFiles []string
}

// maxAutomationDepth limits recursive automation triggering. When an entity
// is created by automation, it can trigger further automations up to this
// depth. Beyond this limit, entities are still created but automations are
// skipped with a warning. This prevents infinite loops from misconfigured
// automations while allowing useful chaining (e.g., ticket → checklist → items).
const (
	maxAutomationDepth   = 50
	storeEventBufferSize = 32
)

// Discover discovers a project from the given start directory and creates
// a workspace with the given script executor. If startDir is empty, it uses
// the current working directory.
//
// For production use, pass script.NewEngine() for Lua support.
// For tests, pass NopScriptExecutor.
func Discover(startDir string, scriptExec ScriptExecutor) (*Workspace, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, err
	}
	return New(fs, ctx, scriptExec)
}

// New creates a workspace over the given filesystem + project paths.
// It loads the metamodel, opens an fsstore, and sets up the automation
// engine and search index.
//
// For production use, pass script.NewEngine() for Lua support.
// For tests, pass NopScriptExecutor.
func New(fs storage.FS, paths *project.Context, scriptExec ScriptExecutor, opts ...Option) (*Workspace, error) {
	exec := scriptExec
	if exec == nil {
		exec = NopScriptExecutor
	}

	meta, schemaFiles, err := metamodel.NewFSLoader(fs, paths.MetamodelPath).Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load metamodel: %w", err)
	}

	// Pre-scan opts for a caller-supplied factory; fall back to the
	// default fsstore factory rooted at the given fs/paths.
	var factory store.Factory
	for _, opt := range opts {
		tmp := &Workspace{}
		opt(tmp)
		if tmp.storeFactory != nil {
			factory = tmp.storeFactory
		}
	}
	if factory == nil {
		factory = &defaultStoreFactory{fs: fs, paths: paths}
	}

	s, openErr := factory.OpenStore(meta)
	if openErr != nil {
		return nil, fmt.Errorf("open store: %w", openErr)
	}
	ws := newWorkspace(fs, paths, meta, exec, opts...)
	ws.syncStore = s
	ws.storeFactory = factory
	ws.schemaFiles = schemaFiles

	// Index entities from the store into the newly-created search index.
	if current := ws.state.Load(); current != nil && current.searchIdx != nil {
		if err := indexStoreEntities(current.searchIdx, s, meta); err != nil {
			slog.Warn("failed to index entities", "error", err)
		}
	}
	return ws, nil
}

// NewBare creates a workspace from a repo + metamodel without loading
// anything from disk. Use this in tests that want to seed fixtures
// through the workspace API (SeedEntityForTest / SeedRelationForTest)
// and in production paths that have already loaded the metamodel.
//
// Optional scriptExec (script.Engine) and Option values can be mixed
// in the variadic arguments; pass a script.Engine for Lua automation,
// otherwise NopScriptExecutor is used.
func NewBare(
	fs storage.FS, paths *project.Context, meta *metamodel.Metamodel, opts ...interface{},
) *Workspace {
	exec := NopScriptExecutor
	var wsOpts []Option
	for _, o := range opts {
		switch v := o.(type) {
		case ScriptExecutor:
			exec = v
		case Option:
			wsOpts = append(wsOpts, v)
		}
	}
	return newWorkspace(fs, paths, meta, exec, wsOpts...)
}

// NewForTestWithStore creates a minimal workspace for testing backed by
// a caller-supplied store.Store. Write operations panic — there is no
// repository.
//
// Use this when the test naturally expresses its fixture as
// store.CreateEntity/CreateRelation calls.
func NewForTestWithStore(s store.Store, meta *metamodel.Metamodel) *Workspace {
	idx, err := search.NewIndex()
	if err != nil {
		panic(fmt.Sprintf("NewForTestWithStore: create search index: %v", err))
	}
	if indexErr := indexStoreEntities(idx, s, meta); indexErr != nil {
		panic(fmt.Sprintf("NewForTestWithStore: index entities: %v", indexErr))
	}

	ws := &Workspace{
		config: project.DefaultConfig(),
		store:  s,
	}
	ws.state.Store(&workspaceState{
		store:     s,
		meta:      meta,
		searchIdx: idx,
	})
	return ws
}

// SeedEntityForTest writes an entity directly into every store this
// workspace tracks (the state-level mirror and the long-lived syncStore,
// if any). Intended for tests to set up fixtures without going through
// the validation/automation stack of the public CRUD methods.
func (w *Workspace) SeedEntityForTest(e *entity.Entity) {
	_ = w.writeEntity(e)
}

// SeedRelationForTest is the relation counterpart to SeedEntityForTest.
func (w *Workspace) SeedRelationForTest(r *entity.Relation) {
	_ = w.writeRelation(r)
}

// storeWatcher is an optional capability a store.Store implementation
// may provide to react to external (out-of-process) edits. fsstore
// implements it; in-memory backends don't.
type storeWatcher interface {
	StartWatching() error
	StopWatching()
}

// defaultStoreFactory builds an fsstore rooted at the given filesystem
// and project paths. Used by workspace.New when the caller does not
// inject their own factory via WithStoreFactory.
type defaultStoreFactory struct {
	fs    storage.FS
	paths *project.Context
}

func (f *defaultStoreFactory) OpenStore(meta *metamodel.Metamodel) (store.Store, error) {
	schemas := make(map[string]store.EntityTypeSchema)
	if meta != nil {
		for name, et := range meta.Entities {
			schemas[name] = store.EntityTypeSchema{
				Plural:        et.Plural,
				PropertyOrder: et.PropertyOrder,
			}
		}
	}
	return fsstore.New(fsstore.Config{
		FS:           f.fs,
		EntitiesDir:  f.paths.EntitiesDir,
		RelationsDir: f.paths.RelationsDir,
		CacheDir:     f.paths.CacheDir,
		Schemas:      schemas,
	})
}

// Option configures a Workspace.
type Option func(*Workspace)

// WithStore sets the store backing this workspace.
func WithStore(s store.Store) Option {
	return func(w *Workspace) { w.store = s }
}

// WithStoreFactory injects a store.Factory used by Sync/Reload to
// (re)build the workspace graph from a store instead of repo.Sync.
// When nil or not supplied, workspace falls back to repo.Sync.
func WithStoreFactory(f store.Factory) Option {
	return func(w *Workspace) { w.storeFactory = f }
}

func newWorkspace(
	fs storage.FS, paths *project.Context, meta *metamodel.Metamodel, scriptExec ScriptExecutor,
	opts ...Option,
) *Workspace {
	var autoEngine *automation.Engine
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
	}

	// Create an empty search index — the workspace has no entities yet
	// at construction time. Reload (or the syncStore's initial sync)
	// will populate it. Failures degrade search but don't block
	// construction; Search() surfaces the nil index as an explicit error.
	var searchIdx *search.Index
	if idx, err := search.NewIndex(); err == nil {
		searchIdx = idx
	} else {
		slog.Warn("failed to create search index", "error", err)
	}

	// Load project config (use defaults if not found or paths is nil).
	var cfg *project.Config
	if paths != nil {
		var err error
		cfg, err = project.LoadConfig(paths)
		if err != nil {
			slog.Warn("failed to load config", "error", err)
			cfg = project.DefaultConfig()
		}
	} else {
		cfg = project.DefaultConfig()
	}

	ws := &Workspace{
		fs:         fs,
		paths:      paths,
		config:     cfg,
		scriptExec: scriptExec,
	}
	for _, opt := range opts {
		opt(ws)
	}

	ws.state.Store(&workspaceState{
		store:      memstore.New(),
		meta:       meta,
		automation: autoEngine,
		searchIdx:  searchIdx,
	})
	return ws
}

// indexStoreEntities (re)indexes every entity in the store into the
// given search index. Errors from the store iterator are surfaced;
// index errors are returned so the caller can decide whether to
// publish a partial index.
func indexStoreEntities(idx *search.Index, s store.Store, meta *metamodel.Metamodel) error {
	if idx == nil || s == nil {
		return nil
	}
	docs := storeSearchDocuments(s, meta)
	return idx.IndexBatch(docs)
}

// syncCountsFromStore reports how many entities and relations the
// workspace currently observes in the given store. Relations with a
// missing endpoint are silently skipped.
func syncCountsFromStore(s store.Store) (entities, relations int) {
	if s == nil {
		return 0, 0
	}
	ctx := context.Background()
	ids := make(map[string]struct{})
	for e, err := range s.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			continue
		}
		ids[e.ID] = struct{}{}
		entities++
	}
	for r, err := range s.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			continue
		}
		if _, ok := ids[r.From]; !ok {
			continue
		}
		if _, ok := ids[r.To]; !ok {
			continue
		}
		relations++
	}
	return entities, relations
}

// writeEntity is the single write-through path for entity upserts. It
// calls CreateEntity (falling back to UpdateEntity on ErrConflict) on
// every store this workspace tracks — the long-lived syncStore and the
// state-level in-memory mirror. The syncStore's fsstore implementation
// handles on-disk persistence; no separate repo write is needed.
func (w *Workspace) writeEntity(e *entity.Entity) error {
	if e == nil {
		return nil
	}
	if err := storeUpsertEntity(w.syncStore, e); err != nil {
		return err
	}
	if s := w.state.Load(); s != nil && s.store != nil && s.store != w.syncStore {
		_ = storeUpsertEntity(s.store, e)
	}
	return nil
}

// deleteEntityStore is the write-through delete path. Cascade is false
// because the workspace's CRUD already deletes incident relations
// explicitly before calling this.
func (w *Workspace) deleteEntityStore(id string) error {
	if id == "" {
		return nil
	}
	if w.syncStore != nil {
		_, err := w.syncStore.DeleteEntity(context.Background(), id, false)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	if s := w.state.Load(); s != nil && s.store != nil && s.store != w.syncStore {
		_, _ = s.store.DeleteEntity(context.Background(), id, false)
	}
	return nil
}

// writeRelation upserts a relation into every store.
func (w *Workspace) writeRelation(r *entity.Relation) error {
	if r == nil {
		return nil
	}
	if err := storeUpsertRelation(w.syncStore, r); err != nil {
		return err
	}
	if s := w.state.Load(); s != nil && s.store != nil && s.store != w.syncStore {
		_ = storeUpsertRelation(s.store, r)
	}
	return nil
}

// deleteRelationStore removes a relation from every store.
func (w *Workspace) deleteRelationStore(from, relType, to string) error {
	if w.syncStore != nil {
		err := w.syncStore.DeleteRelation(context.Background(), from, relType, to)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	if s := w.state.Load(); s != nil && s.store != nil && s.store != w.syncStore {
		_ = s.store.DeleteRelation(context.Background(), from, relType, to)
	}
	return nil
}

// storeUpsertEntity tries CreateEntity first and falls back to
// UpdateEntity when the entity already exists. Skips nil stores.
func storeUpsertEntity(st store.Store, e *entity.Entity) error {
	if st == nil {
		return nil
	}
	ctx := context.Background()
	err := st.CreateEntity(ctx, e)
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrConflict) {
		return st.UpdateEntity(ctx, e)
	}
	return err
}

// storeUpsertRelation tries CreateRelation first and falls back to
// UpdateRelation with the relation's properties/content when the
// relation already exists.
func storeUpsertRelation(st store.Store, r *entity.Relation) error {
	if st == nil {
		return nil
	}
	ctx := context.Background()
	var data *store.RelationData
	if len(r.Properties) > 0 || r.Content != "" {
		data = &store.RelationData{Properties: r.Properties, Content: r.Content}
	}
	_, err := st.CreateRelation(ctx, r.From, r.Type, r.To, data)
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrConflict) {
		update := store.RelationData{}
		if data != nil {
			update = *data
		}
		_, err = st.UpdateRelation(ctx, r.From, r.Type, r.To, update)
		return err
	}
	return err
}

// --- Accessors ---

// Meta returns the current metamodel.
func (w *Workspace) Meta() *metamodel.Metamodel {
	if s := w.state.Load(); s != nil {
		return s.meta
	}
	return nil
}

// Store returns the store backing this workspace. Precedence:
//  1. An external store injected via WithStore (used by the CLI/data-entry
//     wiring that owns the fsstore lifecycle).
//  2. The long-lived syncStore opened by workspace.New (fsstore).
//  3. The state-level in-memory mirror (s.store) — kept in sync via
//     mirrorEntityUpsert/Delete on every CRUD call.
func (w *Workspace) Store() store.Store {
	if w.store != nil {
		return w.store
	}
	if w.syncStore != nil {
		return w.syncStore
	}
	if s := w.state.Load(); s != nil {
		return s.store
	}
	return nil
}

// search runs the legacy word/phrase Bleve query against a single workspace
// state snapshot so the index, the graph it was built from, and the returned
// entities all come from the same epoch. External callers use Searcher()
// instead; this method backs that adapter.
func (w *Workspace) search(words, phrases []string, limit int) ([]*entity.Entity, []float64, error) {
	s := w.state.Load()
	if s == nil || s.searchIdx == nil {
		return nil, nil, fmt.Errorf("search index not available")
	}
	results, err := s.searchIdx.Search(words, phrases, limit)
	if err != nil {
		return nil, nil, err
	}
	ctx := context.Background()
	st := w.Store()
	entities := make([]*entity.Entity, 0, len(results))
	scores := make([]float64, 0, len(results))
	for _, r := range results {
		e, err := st.GetEntity(ctx, r.ID)
		if err != nil {
			continue
		}
		entities = append(entities, e)
		scores = append(scores, r.Score)
	}
	return entities, scores, nil
}

// --- Project accessors ---

// Paths returns the project directory layout.
func (w *Workspace) Paths() *project.Context { return w.paths }

// Config returns the project-config loader (data-entry.yaml, schedules.yaml, ...).
// Returns a no-op loader when the workspace has no filesystem configured
// (rare; happens only in tests that stub the workspace).
func (w *Workspace) Config() config.Loader {
	if w.fs == nil || w.paths == nil {
		return nopConfig{}
	}
	return config.NewFSLoader(w.fs, w.paths.Root)
}

// State returns the per-user state KV (UI state, render caches, scheduler state).
// Returns a no-op KV when the workspace has no filesystem configured.
func (w *Workspace) State() state.KV {
	if w.fs == nil || w.paths == nil {
		return nopState{}
	}
	return state.NewFSKV(w.fs, w.paths.CacheDir)
}

type nopConfig struct{}

func (nopConfig) Load(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("workspace: no repository configured")
}

type nopState struct{}

func (nopState) Get(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("workspace: no repository configured")
}

func (nopState) Put(context.Context, string, []byte) error {
	return fmt.Errorf("workspace: no repository configured")
}

// FindOrphanedTempFiles returns paths of leftover .new temp files in
// the entities/ and relations/ directories. These can arise from
// interrupted writes.
func (w *Workspace) FindOrphanedTempFiles() ([]string, error) {
	if w.fs == nil || w.paths == nil {
		return nil, nil
	}
	var orphaned []string
	orphaned = append(orphaned, findTempFilesInDir(w.fs, w.paths.EntitiesDir)...)
	orphaned = append(orphaned, findTempFilesInDir(w.fs, w.paths.RelationsDir)...)
	return orphaned, nil
}

// CleanupOrphanedTempFiles removes every orphaned .new temp file.
// Returns the number of files cleaned up.
func (w *Workspace) CleanupOrphanedTempFiles() (int, error) {
	orphaned, err := w.FindOrphanedTempFiles()
	if err != nil {
		return 0, err
	}
	for _, path := range orphaned {
		if removeErr := w.fs.Remove(path); removeErr != nil {
			return 0, fmt.Errorf("remove %s: %w", path, removeErr)
		}
	}
	return len(orphaned), nil
}

// findTempFilesInDir walks a directory (recursively) for .new temp files.
func findTempFilesInDir(fs storage.FS, dir string) []string {
	var result []string
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		name := entry.Name()
		path := dir + "/" + name
		if entry.IsDir() {
			result = append(result, findTempFilesInDir(fs, path)...)
		} else if strings.HasSuffix(name, ".new") {
			result = append(result, path)
		}
	}
	return result
}

// --- Lifecycle ---

// Reload reloads the metamodel and re-syncs the graph from disk. This is
// called automatically by the file watcher but is also available for
// programmatic use (after migration, in tests, etc.).
//
// Reload is serialized against concurrent Reload, Sync, and Close via
// reloadMu. Readers (Meta, Search, etc.) never take the lock; they get
// a coherent {graph, meta, automation, searchIdx} snapshot via state.Load().
//
// On success, a single new workspaceState is published atomically. The
// new state contains a freshly-built graph (returned by repo.Sync), the
// new metamodel, a new automation engine, and a new search index. On
// failure, nothing is published — the previous state remains live and
// concurrent readers never observe a broken or partial workspace.
func (w *Workspace) Reload() (entities, relations int, err error) {
	w.reloadMu.Lock()
	defer w.reloadMu.Unlock()

	// Skip reloads after Close: they would resurrect the workspace with
	// fresh resources that no one would close.
	if w.closed.Load() {
		return 0, 0, fmt.Errorf("workspace is closed")
	}

	oldState := w.state.Load()

	newMeta, newSchemaFiles, err := w.MetaLoader().Load(context.Background())
	if err != nil {
		if migration.IsMigrationError(err) {
			return w.reloadKeepingOldMetamodel(oldState)
		}
		return 0, 0, fmt.Errorf("reload metamodel: %w", err)
	}

	// Pick the store that backs the reloaded state. In production the
	// syncStore is authoritative and reflects every on-disk change.
	// For store-less test workspaces we carry the current mirror forward
	// unchanged — only the metamodel-derived state (automation, search
	// index) gets refreshed.
	newStore := w.syncStore
	if newStore == nil && oldState != nil {
		newStore = oldState.store
	}
	if newStore == nil {
		newStore = memstore.New()
	}
	entitiesLoaded, relationsLoaded := syncCountsFromStore(newStore)

	// Build a new automation engine from the new metamodel.
	var newAuto *automation.Engine
	if len(newMeta.Automations) > 0 {
		newAuto = automation.NewEngineFromMetamodel(newMeta.Automations)
	}

	// Build the new search index against the new metamodel and the
	// freshly-synced store. Keep the previous index if construction or
	// indexing fails — never drop a working index in favor of a broken one.
	newIdx := w.buildReloadSearchIndex(newStore, newMeta, oldState)

	// Publish the new state atomically. Readers that call state.Load()
	// after this point see a fully coherent {newStore, newMeta, newAuto,
	// newIdx} tuple — no torn reads, no out-of-order publication.
	w.state.Store(&workspaceState{
		store:      newStore,
		meta:       newMeta,
		automation: newAuto,
		searchIdx:  newIdx,
	})

	// Watch any new schema include files that appeared after reload.
	w.updateSchemaFiles(newSchemaFiles)

	// Close the old search index if it was replaced (not carried over).
	if oldState != nil && oldState.searchIdx != nil && oldState.searchIdx != newIdx {
		if closeErr := oldState.searchIdx.Close(); closeErr != nil {
			slog.Warn("failed to close old search index", "error", closeErr)
		}
	}

	return entitiesLoaded, relationsLoaded, nil
}

// reloadKeepingOldMetamodel handles the migration-error path of Reload:
// the metamodel file changed in a way that requires `rela migrate`, but
// we still want to pick up entity-file changes from disk. We re-sync the
// graph with the OLD metamodel and rebuild the search index against the
// new graph, so the published state remains internally consistent.
//
// Caller must hold reloadMu.
func (w *Workspace) reloadKeepingOldMetamodel(
	oldState *workspaceState,
) (entities, relations int, err error) {
	slog.Warn("metamodel needs migration, skipping metamodel reload: run 'rela migrate'")
	if oldState == nil {
		return 0, 0, fmt.Errorf("reload: no current metamodel and new one needs migration")
	}

	newStore := w.syncStore
	if newStore == nil {
		newStore = oldState.store
	}
	entitiesLoaded, relationsLoaded := syncCountsFromStore(newStore)

	// Rebuild the search index against the freshly-synced store (with
	// the OLD metamodel) so Search returns consistent results. Without
	// this, the published state would have a new store paired with an
	// index built from the previous state — a torn snapshot.
	newIdx := w.buildReloadSearchIndex(newStore, oldState.meta, oldState)

	w.state.Store(&workspaceState{
		store:      newStore,
		meta:       oldState.meta,
		automation: oldState.automation,
		searchIdx:  newIdx,
	})

	if oldState.searchIdx != nil && oldState.searchIdx != newIdx {
		if closeErr := oldState.searchIdx.Close(); closeErr != nil {
			slog.Warn("failed to close old search index", "error", closeErr)
		}
	}

	return entitiesLoaded, relationsLoaded, nil
}

// buildReloadSearchIndex creates a fresh search index for a reload, built
// from entities in the given store. On any failure (creating the index,
// batch-indexing the documents), it returns the old index from oldState
// instead of dropping indexing entirely. Caller must hold reloadMu.
func (w *Workspace) buildReloadSearchIndex(
	st store.Store, newMeta *metamodel.Metamodel, oldState *workspaceState,
) *search.Index {
	carryOver := func() *search.Index {
		if oldState == nil {
			return nil
		}
		return oldState.searchIdx
	}

	candidate, err := search.NewIndex()
	if err != nil {
		slog.Warn("failed to create search index during reload; keeping previous", "error", err)
		return carryOver()
	}

	docs := storeSearchDocuments(st, newMeta)
	if err := candidate.IndexBatch(docs); err != nil {
		slog.Warn("failed to index entities during reload; keeping previous index", "error", err)
		if closeErr := candidate.Close(); closeErr != nil {
			slog.Warn("failed to close partial search index", "error", closeErr)
		}
		return carryOver()
	}

	return candidate
}

// --- Type resolution ---

// ResolveEntityType resolves a type name (alias, plural) to its canonical
// name and definition.
func (w *Workspace) ResolveEntityType(typeName string) (string, *metamodel.EntityDef, error) {
	meta := w.Meta()

	// Exact match or alias.
	resolved := meta.ResolveAlias(strings.TrimSpace(typeName))
	if def, ok := meta.GetEntityDef(resolved); ok {
		return resolved, def, nil
	}

	// Strip common plural suffixes.
	suffixes := []string{"ies", "es", "s"}
	replacements := []string{"y", "", ""}
	for i, suffix := range suffixes {
		if strings.HasSuffix(typeName, suffix) {
			singular := strings.TrimSuffix(typeName, suffix) + replacements[i]
			resolved = meta.ResolveAlias(singular)
			if def, ok := meta.GetEntityDef(resolved); ok {
				return resolved, def, nil
			}
		}
	}

	return "", nil, fmt.Errorf("unknown entity type: %s", typeName)
}

// --- ID generation ---

// GenerateID generates the next ID for the given entity type. If prefix is
// non-empty it is used instead of the default prefix from the metamodel.
func (w *Workspace) GenerateID(entityType, prefix string) (string, error) {
	s := w.state.Load()
	if s == nil {
		return "", fmt.Errorf("workspace not initialized")
	}
	entityDef, ok := s.meta.GetEntityDef(entityType)
	if !ok {
		return "", fmt.Errorf("unknown entity type: %s", entityType)
	}
	if entityDef.IsManualID() {
		return "", fmt.Errorf("entity type %s uses manual IDs", entityType)
	}
	if prefix == "" {
		prefixes := entityDef.GetIDPrefixes()
		if len(prefixes) == 0 {
			return "", fmt.Errorf("no ID prefixes defined for type %s", entityType)
		}
		prefix = prefixes[0]
	}

	existingIDs := w.collectAllIDs()
	if entityDef.IsShortID() {
		return entity.GenerateShortID(existingIDs, prefix, len(existingIDs), entityDef.GetIDCaps()), nil
	}
	return entity.GenerateNextID(existingIDs, prefix), nil
}

// collectAllIDs returns every entity ID currently in the workspace's
// store. Used for ID generation where we need the full existing set.
func (w *Workspace) collectAllIDs() []string {
	ctx := context.Background()
	ids := make([]string, 0)
	for e, err := range w.Store().ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			return ids
		}
		ids = append(ids, e.ID)
	}
	return ids
}

// --- Entity operations ---

// CreateOptions configures entity creation.
type CreateOptions struct {
	ID         string                 // empty = auto-generate
	Prefix     string                 // override default ID prefix (ignored when ID is set)
	Properties map[string]interface{} // property values
	Content    string                 // markdown body
}

// CreateResult contains side-effects from entity creation.
type CreateResult struct {
	AutomationWarnings []string
	AutomationErrors   []string
	RelationsCreated   []*entity.Relation
	EntitiesCreated    []*entity.Entity
}

// CreateEntity generates an ID (unless provided), applies templates and
// defaults, validates, writes to disk, updates the graph, and runs
// automation.
//
// Captures a single workspace state snapshot at entry. Callers must
// serialize CreateEntity against concurrent Reload via an external mutex
// (App.writeMu in the data-entry server).
func (w *Workspace) createEntity(entityType string, opts CreateOptions) (*entity.Entity, *CreateResult, error) {
	s := w.state.Load()
	if s == nil {
		return nil, nil, fmt.Errorf("workspace not initialized")
	}

	// Check for duplicates if custom ID provided.
	if opts.ID != "" {
		if _, err := w.Store().GetEntity(context.Background(), opts.ID); err == nil {
			return nil, nil, fmt.Errorf("entity with ID %s already exists", opts.ID)
		}
	}

	// Create entity using core logic (no automation yet - we run it after for pre-validation changes).
	entity, err := w.createEntityCore(entityType, createEntityCoreOpts{
		ID:         opts.ID,
		IDPrefix:   opts.Prefix,
		Properties: opts.Properties,
		Content:    opts.Content,
	})
	if err != nil {
		return nil, nil, err
	}

	// Run automation and apply property changes.
	result := &CreateResult{}
	var autoResult *automation.Result
	if s.automation != nil {
		autoResult = s.automation.Process(automation.Event{
			Type:   automation.EventEntityCreated,
			Entity: entity,
		})
		// Apply property changes.
		if len(autoResult.PropertiesSet) > 0 {
			for prop, val := range autoResult.PropertiesSet {
				entity.SetString(prop, val)
			}
			// Re-write entity with automation-set properties.
			if err := w.writeEntity(entity); err != nil {
				return nil, nil, fmt.Errorf("write entity after automation: %w", err)
			}
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors
	}

	// Apply automation side effects (relations, entities, Lua) after entity is written.
	if autoResult != nil {
		effects := w.applyAutomationSideEffects(entity, nil, autoResult)
		result.RelationsCreated = effects.RelationsCreated
		result.EntitiesCreated = effects.EntitiesCreated
		result.AutomationErrors = append(result.AutomationErrors, effects.Errors...)
		result.AutomationWarnings = append(result.AutomationWarnings, effects.Warnings...)
	}

	return entity, result, nil
}

// UpdateResult contains side-effects from entity update.
type UpdateResult struct {
	AutomationWarnings []string
	AutomationErrors   []string
	RelationsCreated   []*entity.Relation
	EntitiesCreated    []*entity.Entity
}

// UpdateEntity validates and writes an existing entity, runs automation,
// and mirrors to the in-memory store(s).
//
// Captures a single workspace state snapshot at entry; all reads of meta,
// automation, and graph during this call use that snapshot. Callers must
// serialize UpdateEntity against concurrent Reload via an external mutex
// (App.writeMu in the data-entry server) so the snapshot the method
// holds matches the workspace's current state until the method returns.
func (w *Workspace) updateEntity(entity, oldEntity *entity.Entity) (*UpdateResult, error) {
	s := w.state.Load()
	if s == nil {
		return nil, fmt.Errorf("workspace not initialized")
	}
	meta := s.meta

	// Validate.
	if errs := meta.ValidateEntity(entity.ID, entity.Type, entity.Properties); len(errs) > 0 {
		return nil, newValidationError(errs)
	}

	result := &UpdateResult{}

	// Run automation to get property changes and side effects.
	var autoResult *automation.Result
	if s.automation != nil && oldEntity != nil {
		autoResult = s.automation.Process(automation.Event{
			Type:      automation.EventEntityUpdated,
			Entity:    entity,
			OldEntity: oldEntity,
		})
		for prop, val := range autoResult.PropertiesSet {
			entity.SetString(prop, val)
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors
	}

	// Write to disk + mirror BEFORE side effects. This ensures Lua scripts
	// can modify entities without being overwritten.
	if err := w.writeEntity(entity); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}

	// Apply automation side effects (relations, entities, Lua) AFTER entity is written.
	if autoResult != nil {
		effects := w.applyAutomationSideEffects(entity, oldEntity, autoResult)
		result.RelationsCreated = effects.RelationsCreated
		result.EntitiesCreated = effects.EntitiesCreated
		result.AutomationErrors = append(result.AutomationErrors, effects.Errors...)
		result.AutomationWarnings = append(result.AutomationWarnings, effects.Warnings...)
	}

	return result, nil
}

// DeleteResult contains info about what was deleted.
type DeleteResult struct {
	RelationsDeleted int
}

// ErrHasRelations is returned by DeleteEntity when cascade is false but
// the entity has relations.
var ErrHasRelations = fmt.Errorf("entity has relations; set cascade=true to delete")

// DeleteEntity removes an entity and optionally cascades to its relations.
func (w *Workspace) deleteEntity(_, id string, cascade bool) (*DeleteResult, error) {
	s := w.state.Load()
	if s == nil {
		return nil, fmt.Errorf("workspace not initialized")
	}

	ctx := context.Background()
	st := w.Store()
	if _, err := st.GetEntity(ctx, id); err != nil {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	incoming := collectRelations(st, store.RelationQuery{EntityID: id, Direction: store.DirectionIncoming})
	outgoing := collectRelations(st, store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing})
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return nil, ErrHasRelations
	}

	result := &DeleteResult{}

	// Delete relations first.
	for _, rel := range incoming {
		if err := w.deleteRelationStore(rel.From, rel.Type, rel.To); err != nil {
			slog.Warn("failed to delete relation", "from", rel.From, "type", rel.Type, "to", rel.To, "error", err)
		}
		result.RelationsDeleted++
	}
	for _, rel := range outgoing {
		if err := w.deleteRelationStore(rel.From, rel.Type, rel.To); err != nil {
			slog.Warn("failed to delete relation", "from", rel.From, "type", rel.Type, "to", rel.To, "error", err)
		}
		result.RelationsDeleted++
	}

	// Delete entity.
	if err := w.deleteEntityStore(id); err != nil {
		return nil, fmt.Errorf("delete entity: %w", err)
	}

	return result, nil
}

// createEntityCoreOpts configures core entity creation.
type createEntityCoreOpts struct {
	ID              string                 // Custom ID (empty = auto-generate)
	IDPrefix        string                 // Prefix for auto-generated ID
	TemplateVariant string                 // Template variant name (empty = default template)
	Properties      map[string]interface{} // Properties to set
	Content         string                 // Body content
}

// createEntityCore creates an entity without running automations.
// This is the shared creation logic used by CreateEntity and automation processing.
func (w *Workspace) createEntityCore(entityType string, opts createEntityCoreOpts) (*entity.Entity, error) {
	meta := w.Meta()
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}

	// Resolve ID.
	entityID := opts.ID
	if entityID == "" {
		id, err := w.GenerateID(entityType, opts.IDPrefix)
		if err != nil {
			return nil, err
		}
		entityID = id
	} else {
		if err := entity.ValidateID(entityID); err != nil {
			return nil, err
		}
	}

	e := entity.New(entityID, entityType)

	// Apply template defaults (use variant if specified).
	tmpl, err := w.Templater().EntityTemplate(context.Background(), entityType, opts.TemplateVariant)
	if err != nil {
		return nil, fmt.Errorf("load template: %w", err)
	}
	// If a variant was explicitly specified but not found, that's an error.
	if opts.TemplateVariant != "" && tmpl == nil {
		return nil, fmt.Errorf("template variant %q not found for entity type %s", opts.TemplateVariant, entityType)
	}
	if tmpl != nil {
		e.Properties, e.Content = templating.ApplyEntity(e.Properties, e.Content, tmpl)
	}

	// Apply provided properties (override template defaults).
	for k, v := range opts.Properties {
		e.Properties[k] = v
	}

	// Set body content.
	if opts.Content != "" {
		e.Content = opts.Content
	}

	// Set default status if not set.
	if e.GetString("status") == "" {
		e.SetString("status", entityDef.GetDefaultStatus(meta))
	}

	// Validate.
	if errs := meta.ValidateEntity(e.ID, e.Type, e.Properties); len(errs) > 0 {
		return nil, newValidationError(errs)
	}

	// Persist to every store (mirror + syncStore + disk via fsstore).
	if err := w.writeEntity(e); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}

	return e, nil
}

// automationSideEffects holds entities and relations created by automation.
type automationSideEffects struct {
	RelationsCreated []*entity.Relation
	EntitiesCreated  []*entity.Entity
	Errors           []string
	Warnings         []string
}

// findExistingRelationTarget finds an existing entity of the given type that is
// the target of a relation from the source entity with the given relation type.
// Returns nil if no such entity exists.
func (w *Workspace) findExistingRelationTarget(sourceID, relationType, targetType string) *entity.Entity {
	ctx := context.Background()
	st := w.Store()
	for rel, err := range st.ListRelations(ctx, store.RelationQuery{
		EntityID:  sourceID,
		Direction: store.DirectionOutgoing,
		Type:      relationType,
	}) {
		if err != nil {
			continue
		}
		target, err := st.GetEntity(ctx, rel.To)
		if err != nil {
			continue
		}
		if target.Type == targetType {
			return target
		}
	}
	return nil
}

// automationQueueItem represents a pending automation result to process.
type automationQueueItem struct {
	trigger    *entity.Entity
	autoResult *automation.Result
}

// applyAutomationSideEffects processes automation results iteratively using a BFS queue.
// This avoids deep recursion and provides clear iteration limits.
func (w *Workspace) applyAutomationSideEffects(
	triggerEntity *entity.Entity,
	oldEntity *entity.Entity,
	autoResult *automation.Result,
) *automationSideEffects {
	effects := &automationSideEffects{}

	// BFS queue of pending automation results to process.
	queue := []automationQueueItem{{triggerEntity, autoResult}}
	iterations := 0

	for len(queue) > 0 && iterations < maxAutomationDepth {
		// Pop from front (BFS order - process all items at depth N before depth N+1).
		item := queue[0]
		queue = queue[1:]
		iterations++

		// Process Lua scripts for this trigger.
		w.executeLuaActions(item.trigger, oldEntity, item.autoResult.LuaToExecute, effects)

		// Process relations for this trigger.
		w.applyRelationCreations(item.trigger, item.autoResult.RelationsToCreate, effects)

		// Collect warnings/errors from this automation result.
		effects.Warnings = append(effects.Warnings, item.autoResult.Warnings...)
		effects.Errors = append(effects.Errors, item.autoResult.Errors...)

		// Process entity creations and collect any new queue items.
		newItems := w.processEntityCreations(item.trigger, item.autoResult.EntitiesToCreate, effects)
		queue = append(queue, newItems...)
	}

	// Warn if we hit the limit with work remaining.
	if len(queue) > 0 {
		effects.Warnings = append(effects.Warnings,
			fmt.Sprintf("automation iteration limit (%d) reached; %d pending items skipped",
				maxAutomationDepth, len(queue)))
	}

	return effects
}

// processEntityCreations handles entity creation from automation and returns new queue items.
func (w *Workspace) processEntityCreations(
	trigger *entity.Entity,
	toCreateList []automation.EntityToCreate,
	effects *automationSideEffects,
) []automationQueueItem {
	var newItems []automationQueueItem
	meta := w.Meta()

	for _, toCreate := range toCreateList {
		if skip := w.handleIfExists(trigger, toCreate, effects); skip {
			continue
		}

		// Create entity (no automation yet).
		created, createErr := w.createEntityCore(toCreate.Type, createEntityCoreOpts{
			TemplateVariant: toCreate.Template,
			Properties:      toCreate.Properties,
		})
		if createErr != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("failed to create automation entity %s: %v", toCreate.Type, createErr))

			continue
		}
		effects.EntitiesCreated = append(effects.EntitiesCreated, created)

		// Create relation from trigger if specified.
		if toCreate.RelationFromTrigger != "" {
			w.createTriggerRelation(trigger, created, toCreate.RelationFromTrigger, effects)
		}

		// Run automation on newly created entity.
		newItem := w.runCreatedEntityAutomation(created, meta, effects)
		if newItem != nil {
			newItems = append(newItems, *newItem)
		}
	}

	return newItems
}

// runCreatedEntityAutomation runs automation on a newly created entity and returns a queue item if needed.
func (w *Workspace) runCreatedEntityAutomation(
	created *entity.Entity,
	_ *metamodel.Metamodel,
	effects *automationSideEffects,
) *automationQueueItem {
	s := w.state.Load()
	if s == nil || s.automation == nil {
		return nil
	}

	newAutoResult := s.automation.Process(automation.Event{
		Type:   automation.EventEntityCreated,
		Entity: created,
	})

	// Apply property changes from automation.
	if len(newAutoResult.PropertiesSet) > 0 {
		for prop, val := range newAutoResult.PropertiesSet {
			created.SetString(prop, val)
		}
		// Re-write entity with updated properties.
		if err := w.writeEntity(created); err != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("failed to update automation entity %s: %v", created.ID, err))
		}
	}

	// Return queue item if there's more work to do.
	hasWork := len(newAutoResult.EntitiesToCreate) > 0 || len(newAutoResult.RelationsToCreate) > 0 ||
		len(newAutoResult.LuaToExecute) > 0 ||
		len(newAutoResult.Warnings) > 0 || len(newAutoResult.Errors) > 0
	if hasWork {
		return &automationQueueItem{created, newAutoResult}
	}

	return nil
}

// applyRelationCreations creates relations from automation results.
func (w *Workspace) applyRelationCreations(
	triggerEntity *entity.Entity,
	relations []*entity.Relation,
	effects *automationSideEffects,
) {
	meta := w.Meta()

	for _, rel := range relations {
		rel.From = triggerEntity.ID

		targetEntity, err := w.Store().GetEntity(context.Background(), rel.To)
		if err != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("automation relation target not found: %s", rel.To))
			continue
		}
		if err := meta.ValidateRelation(rel.Type, triggerEntity.Type, targetEntity.Type); err != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("automation relation invalid: %v", err))
			continue
		}

		if err := w.writeRelationCore(rel); err != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("failed to create automation relation: %v", err))
			continue
		}
		effects.RelationsCreated = append(effects.RelationsCreated, rel)
	}
}

// executeLuaActions executes Lua scripts from automation results.
func (w *Workspace) executeLuaActions(
	entity *entity.Entity,
	oldEntity *entity.Entity,
	luaActions []automation.LuaToExecute,
	effects *automationSideEffects,
) {
	if len(luaActions) == 0 {
		return
	}

	// Build script context once for all actions
	ctx := &scriptContextImpl{
		workspace:   w,
		meta:        w.Meta(),
		projectRoot: w.paths.Root,
		entity:      entity,
		oldEntity:   oldEntity,
	}

	for _, action := range luaActions {
		var err error

		switch {
		case action.Code != "":
			// Inline script code
			err = w.scriptExec.ExecuteCode(action.Code, ctx)
		case action.FilePath != "":
			// Script file from scripts/ directory
			err = w.scriptExec.ExecuteFile(action.FilePath, ctx)
		default:
			// Empty action - skip
			continue
		}

		if err != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("script execution error: %s", err.Error()))
		}
	}
}

// handleIfExists checks if_exists behavior for entity creation.
// Returns true if the entity creation should be skipped.
func (w *Workspace) handleIfExists(
	triggerEntity *entity.Entity,
	toCreate automation.EntityToCreate,
	effects *automationSideEffects,
) bool {
	if toCreate.RelationFromTrigger == "" {
		return false
	}

	existingTarget := w.findExistingRelationTarget(
		triggerEntity.ID, toCreate.RelationFromTrigger, toCreate.Type)

	if existingTarget == nil {
		return false
	}

	switch toCreate.IfExists {
	case automation.IfExistsSkip:
		effects.EntitiesCreated = append(effects.EntitiesCreated, existingTarget)
		return true
	case automation.IfExistsError:
		effects.Errors = append(effects.Errors,
			fmt.Sprintf("entity already exists via %s relation: %s",
				toCreate.RelationFromTrigger, existingTarget.ID))
		return true
	case automation.IfExistsReplace:
		if _, err := w.deleteEntity(existingTarget.Type, existingTarget.ID, true); err != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("failed to delete existing entity for replace: %v", err))
			return true
		}
	default:
		effects.Errors = append(effects.Errors,
			fmt.Sprintf("unknown if_exists value %q, skipping entity creation", toCreate.IfExists))
		return true
	}
	return false
}

// createTriggerRelation creates a relation from the trigger entity to a newly created entity.
func (w *Workspace) createTriggerRelation(
	triggerEntity, created *entity.Entity,
	relationType string,
	effects *automationSideEffects,
) {
	meta := w.Meta()

	if err := meta.ValidateRelation(relationType, triggerEntity.Type, created.Type); err != nil {
		effects.Errors = append(effects.Errors,
			fmt.Sprintf("automation relation invalid: %v", err))
		return
	}

	rel := entity.NewRelation(triggerEntity.ID, relationType, created.ID)
	if err := w.writeRelationCore(rel); err != nil {
		effects.Errors = append(effects.Errors,
			fmt.Sprintf("failed to create automation relation: %v", err))
		return
	}
	effects.RelationsCreated = append(effects.RelationsCreated, rel)
}

// --- Relation operations ---

// writeRelationCore persists a relation to every store. Shared by
// CreateRelation and automation processing.
func (w *Workspace) writeRelationCore(rel *entity.Relation) error {
	if err := w.writeRelation(rel); err != nil {
		return fmt.Errorf("write relation: %w", err)
	}
	return nil
}

// CreateRelationOptions configures optional settings for relation creation.
type CreateRelationOptions struct {
	Properties map[string]interface{} // property values for the relation
	Content    string                 // markdown body content for the relation
}

// CreateRelation validates both endpoints exist, checks for duplicates,
// validates against the metamodel, writes to disk, and mirrors to the store.
func (w *Workspace) createRelation(from, relType, to string, opts ...CreateRelationOptions) (*entity.Relation, error) {
	s := w.state.Load()
	if s == nil {
		return nil, fmt.Errorf("workspace not initialized")
	}

	ctx := context.Background()
	st := w.Store()
	fromEntity, err := st.GetEntity(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("source entity not found: %s", from)
	}
	toEntity, err := st.GetEntity(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("target entity not found: %s", to)
	}

	// Validate relation type.
	if vErr := s.meta.ValidateRelation(relType, fromEntity.Type, toEntity.Type); vErr != nil {
		return nil, fmt.Errorf("invalid relation: %w", vErr)
	}

	// Check for duplicates.
	if _, gErr := st.GetRelation(ctx, from, relType, to); gErr == nil {
		return nil, fmt.Errorf("relation already exists: %s --%s--> %s", from, relType, to)
	}

	rel := entity.NewRelation(from, relType, to)

	// Apply template if available.
	tmpl, err := w.Templater().RelationTemplate(context.Background(), relType)
	if err != nil {
		return nil, fmt.Errorf("load relation template: %w", err)
	}
	if tmpl != nil {
		rel.Properties = templating.ApplyRelation(rel.Properties, tmpl)
	}

	// Apply caller-provided properties and content (override template defaults).
	if len(opts) > 0 {
		if len(opts[0].Properties) > 0 && rel.Properties == nil {
			rel.Properties = make(map[string]interface{})
		}
		for k, v := range opts[0].Properties {
			rel.Properties[k] = v
		}
		if opts[0].Content != "" {
			rel.Content = opts[0].Content
		}
	}

	if err := w.writeRelationCore(rel); err != nil {
		return nil, err
	}

	return rel, nil
}

// UpdateRelation updates properties on an existing relation.
func (w *Workspace) updateRelation(from, relType, to string, opts CreateRelationOptions) (*entity.Relation, error) {
	rel, err := w.Store().GetRelation(context.Background(), from, relType, to)
	if err != nil {
		return nil, fmt.Errorf("relation not found: %s --%s--> %s", from, relType, to)
	}

	// Merge properties
	if rel.Properties == nil {
		rel.Properties = make(map[string]interface{})
	}
	for k, v := range opts.Properties {
		rel.Properties[k] = v
	}
	if opts.Content != "" {
		rel.Content = opts.Content
	}

	if err := w.writeRelationCore(rel); err != nil {
		return nil, err
	}

	return rel, nil
}

// DeleteRelation removes a relation from disk and the in-memory store(s).
func (w *Workspace) deleteRelation(from, relType, to string) error {
	if err := w.deleteRelationStore(from, relType, to); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	return nil
}

// --- Rename ---

// --- File watching ---

// WatchOptions configures the file watcher.
type WatchOptions struct {
	// ExtraFiles lists additional files to watch (e.g., data-entry.yaml).
	ExtraFiles []string
	// ExtraDirs lists additional directories to watch (e.g., metamodel/).
	ExtraDirs []string
	// OnChange is called after the workspace has handled data-file changes
	// (entities, relations, extras) via a graph sync or notify-only pass.
	// Metamodel reloads fire OnMetaReload instead — they don't carry the
	// file paths that OnChange listeners usually key off of.
	OnChange func(events []ChangeEvent)
	// OnMetaReload is called after the workspace has reloaded the metamodel
	// in response to a schema-file change. Consumers that need to rebuild
	// derived state (palette, styles, openapi, ...) hook in here.
	OnMetaReload func()
}

// StartWatching begins watching for file changes.
//
//   - Metamodel changes flow through the metamodel loader's Subscribe
//     and fire opts.OnMetaReload.
//   - Entity/relation changes come from the store's own event stream
//     (started by Workspace.New when a store factory is configured) and
//     fire opts.OnChange.
//   - Extra files/dirs (views.yaml, etc.) are watched via a separate
//     storage.Watcher.
func (w *Workspace) StartWatching(opts WatchOptions) error {
	// Subscribe to metamodel changes via the loader.
	if sub, ok := w.MetaLoader().(metamodel.Subscriber); ok {
		stop, err := sub.Subscribe(context.Background(), func() {
			if _, _, reloadErr := w.Reload(); reloadErr != nil {
				slog.Error("reload error", "error", reloadErr)
			}
			if opts.OnMetaReload != nil {
				opts.OnMetaReload()
			}
		})
		if err != nil {
			return err
		}
		w.stopSchema = stop
	}

	// Ask the store to start watching its own data files. The interface
	// doesn't mandate this capability, so we feature-test via type
	// assertion — fsstore supports it; memstore and other in-memory
	// backends don't need to.
	if w.syncStore != nil {
		if sw, ok := w.syncStore.(storeWatcher); ok {
			if err := sw.StartWatching(); err != nil {
				slog.Warn("store watcher not started", "error", err)
			}
		}
	}

	// Forward store events as workspace ChangeEvents so consumers that
	// wired up OnChange for the legacy fs watcher keep working.
	if opts.OnChange != nil && w.syncStore != nil {
		ch, cancel := w.syncStore.Subscribe(storeEventBufferSize)
		w.stopStoreEvents = cancel
		go forwardStoreEvents(ch, opts.OnChange)
	}

	// Optional extra watcher for non-store files (views.yaml, etc.).
	if len(opts.ExtraDirs) > 0 || len(opts.ExtraFiles) > 0 {
		watcher, err := storage.NewWatcher(storage.WatchConfig{
			Dirs:       opts.ExtraDirs,
			Files:      opts.ExtraFiles,
			Extensions: []string{".yaml", ".yml"},
			Debounce:   200 * time.Millisecond,
			SkipHidden: true,
			OnChange: func(events []storage.ChangeEvent) {
				if opts.OnChange != nil {
					opts.OnChange(events)
				}
			},
		})
		if err != nil {
			if w.stopSchema != nil {
				w.stopSchema()
				w.stopSchema = nil
			}
			return err
		}
		go watcher.Start()
		w.watcher = watcher
	}
	return nil
}

// forwardStoreEvents drains a store.Event channel and fires onChange
// once per event, shaped as a single-event workspace.ChangeEvent slice.
// The store already debounces writes, so we don't re-batch here.
func forwardStoreEvents(ch <-chan store.Event, onChange func([]ChangeEvent)) {
	for ev := range ch {
		onChange([]ChangeEvent{{
			Path: storeEventPath(ev),
			Op:   storeEventOp(ev),
		}})
	}
}

func storeEventPath(ev store.Event) string {
	if ev.EntityID != "" {
		return ev.EntityType + "/" + ev.EntityID
	}
	return ev.From + "--" + ev.RelationType + "--" + ev.To
}

func storeEventOp(ev store.Event) storage.ChangeOp {
	switch ev.Op {
	case store.EventEntityCreated, store.EventRelationCreated:
		return storage.OpCreate
	case store.EventEntityDeleted, store.EventRelationDeleted:
		return storage.OpDelete
	default:
		return storage.OpModify
	}
}

// updateSchemaFiles records the latest set of metamodel source files.
// The metamodel loader's subscription manages its own watch list — this
// function only tracks what was last loaded, for any code that needs it.
// Caller must hold reloadMu.
func (w *Workspace) updateSchemaFiles(newSchemaFiles []string) {
	w.schemaFiles = newSchemaFiles
}

// StopWatching stops the file watcher. Serialized against Reload/Sync
// via reloadMu to prevent races with updateSchemaFiles.
func (w *Workspace) StopWatching() {
	w.reloadMu.Lock()
	defer w.reloadMu.Unlock()
	w.stopWatchingLocked()
}

// stopWatchingLocked stops the watcher. Caller must hold reloadMu.
func (w *Workspace) stopWatchingLocked() {
	if w.watcher != nil {
		w.watcher.Stop()
		w.watcher = nil
	}
	if w.stopSchema != nil {
		w.stopSchema()
		w.stopSchema = nil
	}
	if w.stopStoreEvents != nil {
		w.stopStoreEvents()
		w.stopStoreEvents = nil
	}
}

// Close releases resources held by the workspace (search index, watcher).
// Close is idempotent and serialized against concurrent Reload/Sync via
// reloadMu; the closed flag prevents any Reload that arrives after Close
// from resurrecting the workspace.
func (w *Workspace) Close() error {
	w.reloadMu.Lock()
	defer w.reloadMu.Unlock()
	w.stopWatchingLocked()
	if w.closed.Swap(true) {
		return nil // already closed
	}
	s := w.state.Load()
	if s != nil && s.searchIdx != nil {
		if err := s.searchIdx.Close(); err != nil {
			return fmt.Errorf("close search index: %w", err)
		}
	}
	// Publish a state with nil searchIdx so that any reader still calling
	// Search() after Close gets the "not available" error path instead of
	// touching the closed index.
	if s != nil {
		w.state.Store(&workspaceState{
			store:      s.store,
			meta:       s.meta,
			automation: s.automation,
			searchIdx:  nil,
		})
	}
	if w.syncStore != nil {
		if sw, ok := w.syncStore.(storeWatcher); ok {
			sw.StopWatching()
		}
		if err := w.syncStore.Close(); err != nil {
			slog.Warn("failed to close sync store", "error", err)
		}
		w.syncStore = nil
	}
	return nil
}

// PauseWatching temporarily suppresses file change events.
func (w *Workspace) PauseWatching() {
	if w.watcher != nil {
		w.watcher.Pause()
	}
}

// ResumeWatching re-enables file change events after PauseWatching.
func (w *Workspace) ResumeWatching() {
	if w.watcher != nil {
		w.watcher.Resume()
	}
}

// --- Filesystem access ---

// FS returns the underlying filesystem for operations that need direct
// file access (e.g., attachment store, writing output files).
func (w *Workspace) FS() storage.FS {
	return w.fs
}

// NormalizeContent normalizes markdown headers in content so the minimum
// level is ## (h2). Returns the normalized content.
func (w *Workspace) NormalizeContent(content string) string {
	return markdown.NormalizeHeaders(content)
}

// --- Search document conversion ---

// entityToSearchDocument converts an entity to a search.Document.
func entityToSearchDocument(e *entity.Entity, meta *metamodel.Metamodel) search.Document {
	return search.Document{
		ID:          e.ID,
		Type:        e.Type,
		Primary:     meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Description: e.Description(),
		Content:     e.Content,
		Properties:  flattenProperties(e.Properties),
	}
}

// storeSearchDocuments iterates a store and produces search documents for
// every entity. Errors from the store iterator are ignored — partial
// indexing is preferable to dropping the whole index.
func storeSearchDocuments(st store.Store, meta *metamodel.Metamodel) []search.Document {
	if st == nil {
		return nil
	}
	docs := make([]search.Document, 0)
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{}) {
		if err != nil {
			continue
		}
		docs = append(docs, entityToSearchDocument(e, meta))
	}
	return docs
}

// flattenProperties extracts all property values as a single searchable string.
func flattenProperties(props map[string]interface{}) string {
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := props[k]
		switch val := v.(type) {
		case string:
			parts = append(parts, val)
		case []string:
			parts = append(parts, val...)
		case []interface{}:
			for _, item := range val {
				if s, ok := item.(string); ok {
					parts = append(parts, s)
				}
			}
		default:
			parts = append(parts, fmt.Sprintf("%v", v))
		}
	}
	return strings.Join(parts, " ")
}
