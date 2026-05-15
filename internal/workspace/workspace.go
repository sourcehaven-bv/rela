// Package workspace provides a stateful domain session that owns the
// authoritative store, metamodel, search index, and (transitionally)
// constructs the [entitymanager.Manager] that runs every write
// pipeline. All writes go through Workspace's EntityManager so that
// persistence, validation, and automation stay coordinated.
//
// **Transitional.** Workspace is being decomposed: the production
// write path lives in [internal/entitymanager], and Workspace is the
// shim that constructs Manager and wires per-call lua transport (see
// [wsScriptRunner]). TKT-64R3 deletes this package once consumers
// (CLI/MCP/dataentry/scheduler) construct their own services.
package workspace

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// ChangeEvent is re-exported from storage so consumers don't need to
// import storage directly for watcher callback signatures.
type ChangeEvent = storage.ChangeEvent

// ChangeOp is re-exported from storage for the same reason as ChangeEvent.
type ChangeOp = storage.ChangeOp

// ScriptExecutor is what Workspace requires from a script executor:
// the Lua-execution surface used by [luaScriptRunner] when wiring
// automation cascades, plus LuaCache() access for callers that build
// Lua runtimes directly (validation rules, MCP lua_eval, CLI flow).
//
// *script.Engine satisfies this interface structurally; tests can
// pass [NopScriptExecutor].
type ScriptExecutor interface {
	// ExecuteCode runs inline script code with entity context.
	ExecuteCode(code string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error

	// ExecuteFile runs a script file from the scripts/ directory.
	ExecuteFile(path string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error

	// LuaCache returns the executor's shared Lua cache, or nil if
	// the executor does not provide one. Callers that build Lua
	// runtimes directly pass this via lua.WithCache so every runtime
	// in the process shares cache state.
	LuaCache() *lua.Cache
}

// NopScriptExecutor is the no-op [ScriptExecutor] for tests that
// don't exercise Lua. Calling its Execute* methods panics, making
// unexpected script execution loud.
var NopScriptExecutor ScriptExecutor = nopScriptExecutor{}

type nopScriptExecutor struct{}

func (nopScriptExecutor) ExecuteCode(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	panic("workspace.NopScriptExecutor: Lua execution not expected in this context")
}

func (nopScriptExecutor) ExecuteFile(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	panic("workspace.NopScriptExecutor: Lua execution not expected in this context")
}

func (nopScriptExecutor) LuaCache() *lua.Cache { return nil }

// Workspace is a stateful domain session that ties together the store
// (persistence), metamodel (schema), [entitymanager.Manager] (the
// write path that runs validation + automation + cascade), and search
// index. All write operations go through Workspace's EntityManager so
// that persistence, validation, and automation stay coordinated.
//
// # Lifecycle
//
// Metamodel, Manager (including its automation engine and cascade
// runner), and search index are constructed once at New and never
// reloaded — schema or automation changes require a restart. The
// search backend is installed as a store.EntityObserver at store
// construction time so every in-process write (Create/Update/
// DeleteEntity through the store API) updates the search index
// synchronously under the store's write lock. External edits to
// on-disk files are only observed once a caller invokes
// StartWatching, which arms fsstore's own file watcher to translate
// filesystem events into store events.
type Workspace struct {
	// fs is the filesystem handle used for all workspace I/O:
	// directory topology (ReadDir, MkdirAll, Stat, Walk) and byte
	// reads/writes for plaintext workspace state. Confidentiality at
	// the sync boundary is handled by git-crypt (or equivalent)
	// outside this process — see docs/guides for the integration
	// story.
	fs storage.FS

	paths      *project.Context
	config     *project.Config
	scriptExec ScriptExecutor

	// Core services — immutable after construction.
	store         store.Store
	meta          *metamodel.Metamodel
	manager       *entitymanager.Manager // the production write path
	searchBackend *bleveindex.Index      // may be nil if index construction failed

	// Derived services — memoised on first access.
	tracerOnce   sync.Once
	tracer       tracer.Tracer
	searcherOnce sync.Once
	searcher     search.Searcher

	// Watcher state (nil when not watching).
	watcher *storage.Watcher

	// storeFactory opens the workspace's authoritative store. In
	// production this builds the fsstore under the project directory;
	// tests can inject a different factory via WithStoreFactory.
	storeFactory store.Factory
}

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

	meta, _, err := metamodel.NewFSLoader(fs, paths.MetamodelPath).Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load metamodel: %w", err)
	}

	// Pre-scan opts for a caller-supplied factory; fall back to
	// app.FSFactory rooted at the given fs/paths.
	var factory store.Factory
	for _, opt := range opts {
		tmp := &Workspace{}
		opt(tmp)
		if tmp.storeFactory != nil {
			factory = tmp.storeFactory
		}
	}
	if factory == nil {
		factory = &app.FSFactory{FS: fs, Paths: paths}
	}

	// Build the search backend BEFORE opening the store so it can be
	// installed as an observer. The store then calls EntityPut /
	// EntityDelete synchronously on every write; no subscription
	// goroutine is needed. Backend construction failure is non-fatal —
	// Searcher() surfaces an explicit error when queried.
	var searchBackend *bleveindex.Index
	if idx, idxErr := bleveindex.NewMem(); idxErr == nil {
		searchBackend = idx
	} else {
		slog.Warn("failed to create search index", "error", idxErr)
	}
	if observable, ok := factory.(observerWiringFactory); ok && searchBackend != nil {
		observable.AddObserver(searchBackend)
	}

	s, openErr := factory.OpenStore(meta)
	if openErr != nil {
		return nil, fmt.Errorf("open store: %w", openErr)
	}
	ws, err := newWorkspace(fs, paths, meta, exec, s, searchBackend, opts...)
	if err != nil {
		return nil, err
	}
	ws.storeFactory = factory

	// Backfill the initial state: observers are NOT invoked for entities
	// already on disk when the store opens, so iterate ListEntities once
	// after OpenStore to seed the index. Subsequent writes reach the
	// backend via the observer hook installed above.
	//
	// Writes that race this backfill (e.g. an automation firing during
	// construction) are safe: bleve EntityPut is idempotent, so the
	// observer-applied write and the backfill-applied write produce the
	// same final state.
	if searchBackend != nil {
		if err := backfillSearchBackend(context.Background(), searchBackend, s); err != nil {
			slog.Warn("failed to index entities", "error", err)
		}
	}
	return ws, nil
}

// TestOption configures a workspace built via NewForTest.
type TestOption func(*testConfig)

type testConfig struct {
	fs     storage.FS
	paths  *project.Context
	store  store.Store
	script ScriptExecutor
}

// WithFS attaches a filesystem and project paths to the test workspace,
// enabling paths-aware behavior (Paths(), Config(), orphan-temp-file
// scanning). Without this, those accessors return nop/zero values.
func WithFS(fs storage.FS, paths *project.Context) TestOption {
	return func(c *testConfig) {
		c.fs = fs
		c.paths = paths
	}
}

// WithTestStore replaces the default empty memstore with a caller-
// supplied store. The workspace's search index is populated from the
// store's current contents at construction time.
//
// Caveat: a caller-supplied store is NOT auto-wired with the search
// backend as an observer — observer setup must happen at store
// construction, which the workspace cannot retrofit. Initial-state
// backfill still runs, so any entities already in the store appear
// in search results; subsequent writes will not reach the index.
// If you need incremental sync, build the memstore with
// [memstore.WithObserver] yourself and pass that store here, or use
// the default memstore (omit WithTestStore) which wires the observer
// automatically.
func WithTestStore(s store.Store) TestOption {
	return func(c *testConfig) { c.store = s }
}

// WithScript installs a ScriptExecutor so tests that exercise Lua
// automations run real scripts. Without this, NopScriptExecutor is used
// and any automation-triggered script execution panics.
func WithScript(exec ScriptExecutor) TestOption {
	return func(c *testConfig) { c.script = exec }
}

// NewForTest creates a workspace suitable for tests. By default it has
// no filesystem, an empty memstore, and a nop script executor. Use the
// WithFS / WithTestStore / WithScript options to customize.
//
// Write operations go through the store (memstore unless WithTestStore
// is used); SeedEntityForTest / SeedRelationForTest bypass validation
// and automation for fixture setup.
func NewForTest(meta *metamodel.Metamodel, opts ...TestOption) *Workspace {
	cfg := &testConfig{script: NopScriptExecutor}
	for _, opt := range opts {
		opt(cfg)
	}

	// Build the search backend before the store so the default
	// memstore can observe it. WithTestStore-supplied stores skip
	// observer wiring — see WithTestStore's godoc.
	var searchBackend *bleveindex.Index
	if idx, err := bleveindex.NewMem(); err == nil {
		searchBackend = idx
	}

	st := cfg.store
	if st == nil {
		if searchBackend != nil {
			st = memstore.New(memstore.WithObserver(searchBackend))
		} else {
			st = memstore.New()
		}
	}

	ws, err := newWorkspace(cfg.fs, cfg.paths, meta, cfg.script, st, searchBackend)
	if err != nil {
		panic(fmt.Sprintf("NewForTest: %v", err))
	}
	if cfg.store != nil && searchBackend != nil {
		if err := backfillSearchBackend(context.Background(), searchBackend, cfg.store); err != nil {
			panic(fmt.Sprintf("NewForTest: index entities: %v", err))
		}
	}
	return ws
}

// SeedEntityForTest writes an entity directly into the workspace's
// authoritative store. Intended for tests to set up fixtures without
// going through the validation/automation stack of the public CRUD
// methods.
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

// observerWiringFactory is the consumer-side capability the workspace
// looks for on a [store.Factory] to install its search-index backend
// as an [store.EntityObserver] before the store opens. Factories that
// satisfy this interface (today: [*app.FSFactory]) wire the observer
// synchronously into the resulting store; factories that don't (a
// hypothetical remote-only factory) silently degrade to "no
// incremental sync" — Searcher() still works against the initial
// backfill but does not see subsequent writes.
type observerWiringFactory interface {
	AddObserver(store.EntityObserver)
}

// Option configures a Workspace.
type Option func(*Workspace)

// WithStoreFactory injects a store.Factory used by workspace.New to
// open the authoritative store. When nil or not supplied, New falls
// back to the default fsstore factory.
func WithStoreFactory(f store.Factory) Option {
	return func(w *Workspace) { w.storeFactory = f }
}

// newWorkspace is the single-phase Workspace constructor: it owns
// search-index creation, config loading, automation+cascade wiring,
// and [entitymanager.Manager] construction. The store is supplied by
// the caller (fsstore from [New], memstore or caller-supplied from
// [NewForTest]) so Manager binds to the *intended* store rather than
// a placeholder.
func newWorkspace(
	fs storage.FS, paths *project.Context, meta *metamodel.Metamodel,
	scriptExec ScriptExecutor, st store.Store,
	searchBackend *bleveindex.Index,
	opts ...Option,
) (*Workspace, error) {
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
		fs:            fs,
		paths:         paths,
		config:        cfg,
		scriptExec:    scriptExec,
		store:         st,
		meta:          meta,
		searchBackend: searchBackend,
	}
	for _, opt := range opts {
		opt(ws)
	}

	// Wire automation engine + cascade runner. Both are optional; the
	// pair is supplied to Manager together (Manager's constructor
	// enforces "both or neither").
	var autoEngine *automation.Engine
	var cascadeRunner *autocascade.Runner
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
		r, rerr := autocascade.New(autocascade.Deps{Engine: autoEngine})
		if rerr != nil {
			return nil, fmt.Errorf("build autocascade runner: %w", rerr)
		}
		cascadeRunner = r
	}

	// Build the Manager. Workspace's wsEntityManager forwards every
	// write through it; legacy workspace.createEntity / updateEntity
	// / deleteEntity methods were deleted in TKT-IU2S.
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:        ws.store,
		Meta:         ws.meta,
		Templater:    ws.Templater(),
		Automations:  autoEngine,
		Cascade:      cascadeRunner,
		ScriptRunner: &wsScriptRunner{w: ws},
	})
	if err != nil {
		return nil, fmt.Errorf("build entitymanager: %w", err)
	}
	ws.manager = mgr
	return ws, nil
}

// backfillSearchBackend populates a search backend with every entity
// currently in the store. Errors from individual entities are collected
// and returned together so the operator sees the complete picture; a
// partial index is preferable to no index but the caller knows it is
// partial.
func backfillSearchBackend(ctx context.Context, backend *bleveindex.Index, s store.Store) error {
	if backend == nil || s == nil {
		return nil
	}
	entities := make([]*entity.Entity, 0)
	var listErrs []error
	for e, err := range s.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			listErrs = append(listErrs, err)
			continue
		}
		entities = append(entities, e)
	}
	indexed, indexErr := backend.IndexBatch(entities)
	if len(listErrs) == 0 && indexErr == nil {
		return nil
	}
	skipped := len(entities) - indexed
	return fmt.Errorf("backfill indexed %d entities, skipped %d, list errors: %v, index error: %w",
		indexed, skipped, listErrs, indexErr)
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
// the workspace's authoritative store. In production the store is an
// fsstore which persists to disk; in tests it is an in-memory store.
func (w *Workspace) writeEntity(e *entity.Entity) error {
	if e == nil {
		return nil
	}
	return storeUpsertEntity(w.currentStore(), e)
}

// writeRelation upserts a relation into the authoritative store.
func (w *Workspace) writeRelation(r *entity.Relation) error {
	if r == nil {
		return nil
	}
	return storeUpsertRelation(w.currentStore(), r)
}

// currentStore returns the authoritative store for reads and writes.
// Equivalent to Store(), but kept as a short private helper for the
// write paths above.
func (w *Workspace) currentStore() store.Store {
	return w.Store()
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
func (w *Workspace) Meta() *metamodel.Metamodel { return w.meta }

// Store returns the store backing this workspace. In production that is
// the fsstore opened by workspace.New; in tests it is the store injected
// via NewForTest.
func (w *Workspace) Store() store.Store { return w.store }

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
// Returns a no-op KV when the workspace has no filesystem or cache directory
// configured. A non-empty CacheDir with an invalid value panics — this
// indicates programmer error (the CacheDir was populated but malformed),
// which should fail loud rather than silently degrade to nopState.
func (w *Workspace) State() state.KV {
	if w.fs == nil || w.paths == nil || w.paths.CacheDir == "" {
		return nopState{}
	}
	rfs, err := storage.NewRootedFS(w.fs, w.paths.CacheDir)
	if err != nil {
		panic(fmt.Errorf("workspace: invalid state root %q: %w", w.paths.CacheDir, err))
	}
	return state.NewFSKV(rfs)
}

type nopConfig struct{}

func (nopConfig) Load(context.Context, string) ([]byte, error) {
	return nil, errors.New("workspace: no repository configured")
}

type nopState struct{}

func (nopState) Get(context.Context, string) ([]byte, error) {
	return nil, errors.New("workspace: no repository configured")
}

func (nopState) Put(context.Context, string, []byte) error {
	return errors.New("workspace: no repository configured")
}

func (nopState) Delete(context.Context, string) error {
	return errors.New("workspace: no repository configured")
}

// FindOrphanedTempFiles returns paths of leftover .new temp files in
// the entities/ and relations/ directories. These can arise from
// interrupted writes.
func (w *Workspace) FindOrphanedTempFiles() ([]string, error) {
	if w.fs == nil || w.paths == nil {
		return nil, nil
	}
	orphaned := make([]string, 0) //nolint:prealloc // capacity unknown
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
	entityDef, ok := w.meta.GetEntityDef(entityType)
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

// WatchOptions configures the file watcher.
type WatchOptions struct {
	// ExtraFiles lists additional files to watch (e.g., data-entry.yaml).
	ExtraFiles []string
	// ExtraDirs lists additional directories to watch (e.g., metamodel/).
	ExtraDirs []string
	// OnChange is called when any watched extra file changes. Store
	// events are handled internally (they keep the search index in sync);
	// only consumers of ExtraFiles/ExtraDirs receive events here.
	OnChange func(events []ChangeEvent)
}

// StartWatching begins watching for file changes.
//
//   - Entity/relation changes: the store's own watcher (fsstore) emits
//     events; the workspace subscribes at New() to keep the search index
//     in sync. Nothing is wired here for entity/relation changes.
//   - Extra files/dirs (views.yaml, etc.) are watched via a separate
//     storage.Watcher and fire opts.OnChange.
//
// Metamodel and automation changes are not picked up while the workspace
// is running — restart to apply them.
func (w *Workspace) StartWatching(opts WatchOptions) error {
	// Ask the store to start watching its own data files. The interface
	// doesn't mandate this capability, so we feature-test via type
	// assertion — fsstore supports it; memstore and other in-memory
	// backends don't need to.
	if w.store != nil {
		if sw, ok := w.store.(storeWatcher); ok {
			if err := sw.StartWatching(); err != nil {
				slog.Warn("store watcher not started", "error", err)
			}
		}
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
			return err
		}
		go watcher.Start()
		w.watcher = watcher
	}
	return nil
}

// StopWatching stops the file watcher.
func (w *Workspace) StopWatching() {
	if w.watcher != nil {
		w.watcher.Stop()
		w.watcher = nil
	}
}

// Close releases resources held by the workspace (search index, watcher,
// store). Close is not safe to call concurrently with Workspace methods.
//
// Subsystems close in observer-dependency order: store first, so no
// further observer callbacks can land on the search backend; then the
// backend itself. Every subsystem's close is attempted even if an
// earlier one errored so a partial failure doesn't leak store
// goroutines or watcher resources.
func (w *Workspace) Close() error {
	w.StopWatching()

	if w.store != nil {
		if sw, ok := w.store.(storeWatcher); ok {
			sw.StopWatching()
		}
		if lc, ok := w.store.(store.Lifecycle); ok {
			if err := lc.Close(); err != nil {
				slog.Warn("failed to close store", "error", err)
			}
		}
	}

	var firstErr error
	if w.searchBackend != nil {
		if err := w.searchBackend.Close(); err != nil {
			firstErr = fmt.Errorf("close search index: %w", err)
		}
		w.searchBackend = nil
	}
	return firstErr
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
