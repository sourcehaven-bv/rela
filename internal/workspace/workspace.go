// Package workspace provides a stateful domain session that owns the
// authoritative store, metamodel, and automation engine. All writes go
// through Workspace so that persistence, validation, and automation
// stay coordinated.
package workspace

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// ChangeEvent is re-exported from storage so consumers don't need to
// import storage directly for watcher callback signatures.
type ChangeEvent = storage.ChangeEvent

// ChangeOp is re-exported from storage for the same reason as ChangeEvent.
type ChangeOp = storage.ChangeOp

// ScriptExecutor runs automation scripts with entity context. This follows
// dependency inversion: workspace defines the interface it needs; the script
// package implements it. Workspace stays independent of Lua specifics beyond
// the lua.WriteDeps capability bundle.
//
// The executor is stateless — workspace passes deps and the triggering entity
// pair at execution time. The .rela/ cache dir for AI/secrets is derived from
// deps.ProjectRoot by the executor.
//
// For production, pass script.NewEngine(). For tests, pass NopScriptExecutor.
type ScriptExecutor interface {
	// ExecuteCode runs inline script code with entity context.
	ExecuteCode(code string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error
	// ExecuteFile runs a script file from the scripts/ directory.
	ExecuteFile(path string, deps lua.WriteDeps, newEntity, oldEntity *entity.Entity) error
}

// NopScriptExecutor is a no-op implementation of ScriptExecutor for tests
// that don't trigger Lua automations. It panics if actually called, making
// it obvious when a test unexpectedly triggers Lua execution.
var NopScriptExecutor ScriptExecutor = nopScriptExecutor{}

type nopScriptExecutor struct{}

func (nopScriptExecutor) ExecuteCode(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	panic("NopScriptExecutor: Lua execution not expected in this context")
}

func (nopScriptExecutor) ExecuteFile(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	panic("NopScriptExecutor: Lua execution not expected in this context")
}

// Workspace is a stateful domain session that ties together the store
// (persistence), metamodel (schema), automation engine, and search index.
// All write operations go through Workspace so that persistence,
// validation, and automation stay coordinated.
//
// # Lifecycle
//
// Metamodel, automation engine, and search index are loaded once at New
// and never reloaded — schema or automation changes require a restart.
// The store is self-watching: when an fsstore implementation is used, it
// observes external file edits and emits events. Workspace subscribes to
// those events and keeps the search index in sync automatically.
type Workspace struct {
	// fs is the raw filesystem handle used for directory topology
	// (ReadDir, MkdirAll, Stat, Walk). It is NOT wrapped by the
	// encryption decorator — callers that need sealed byte I/O for
	// data files must use bytesFS instead.
	fs storage.FS

	// bytesFS is the decorated byte-I/O handle: cryptofs.FS on
	// encrypted repos, the raw fs on cleartext repos. Every workspace
	// component that writes plaintext user content (attachments today;
	// future additions like the state/document cache — see C2) MUST
	// use this handle rather than fs, otherwise content lands
	// cleartext on disk even when encryption is enabled.
	bytesFS attachment.BytesFS

	paths      *project.Context
	config     *project.Config
	scriptExec ScriptExecutor

	// Core services — immutable after construction.
	store      store.Store
	meta       *metamodel.Metamodel
	automation *automation.Engine // may be nil if the metamodel has no automations
	searchIdx  *search.Index      // may be nil if index construction failed

	// Derived services — memoised on first access. tracer wraps the store
	// and wsSearcher wraps the workspace; both are cheap wrappers but
	// accessing them from Lua bindings on every call was unnecessary churn.
	tracerOnce   sync.Once
	tracer       tracer.Tracer
	searcherOnce sync.Once
	searcher     search.Searcher

	// Watcher state (nil when not watching).
	watcher    *storage.Watcher
	stopSearch func() // cancels the search-index reindex goroutine

	// storeFactory opens the workspace's authoritative store. In
	// production this builds the fsstore under the project directory;
	// tests can inject a different factory via WithStoreFactory.
	storeFactory store.Factory
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

	s, openErr := factory.OpenStore(meta)
	if openErr != nil {
		return nil, fmt.Errorf("open store: %w", openErr)
	}
	ws := newWorkspace(fs, paths, meta, exec, opts...)
	ws.store = s
	ws.storeFactory = factory

	// Resolve the byte-I/O handle that attachments (and future
	// plaintext-owning components) must use. On encrypted repos this
	// is cryptofs.FS, matching what the factory wired into the store
	// itself. Factories that don't implement bytesOpener (test
	// factories) fall back to the raw fs — acceptable because tests
	// are cleartext.
	if opener, ok := factory.(bytesOpener); ok {
		bytes, bErr := opener.OpenBytesFS()
		if bErr != nil {
			return nil, fmt.Errorf("open bytes fs: %w", bErr)
		}
		ws.bytesFS = bytes
	}

	// Populate the search index from the opened store and subscribe to
	// store events so the index stays current when files change on disk
	// (or when the fsstore emits updates from its own watcher).
	if ws.searchIdx != nil {
		if err := indexStoreEntities(ws.searchIdx, s, meta); err != nil {
			slog.Warn("failed to index entities", "error", err)
		}
		ws.startSearchReindex()
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

	ws := newWorkspace(cfg.fs, cfg.paths, meta, cfg.script)
	if cfg.store != nil {
		ws.store = cfg.store
		if ws.searchIdx != nil {
			if err := indexStoreEntities(ws.searchIdx, cfg.store, meta); err != nil {
				panic(fmt.Sprintf("NewForTest: index entities: %v", err))
			}
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

// Option configures a Workspace.
type Option func(*Workspace)

// WithStoreFactory injects a store.Factory used by workspace.New to
// open the authoritative store. When nil or not supplied, New falls
// back to the default fsstore factory.
func WithStoreFactory(f store.Factory) Option {
	return func(w *Workspace) { w.storeFactory = f }
}

// bytesOpener is the optional interface a store.Factory can
// implement to hand back the decorated byte-I/O handle (cryptofs on
// encrypted repos, raw FS otherwise). Workspace uses it to keep
// attachment I/O consistent with the store's encryption behavior.
// The returned handle is deliberately the same subset of methods
// attachment.BytesFS cares about (ReadFile/WriteFile/Remove/Stat);
// fsstore.StoreFS is a superset and is accepted implicitly.
// Test factories that don't encrypt can skip implementing this —
// Workspace falls back to the raw fs.
type bytesOpener interface {
	OpenBytesFS() (attachment.BytesFS, error)
}

func newWorkspace(
	fs storage.FS, paths *project.Context, meta *metamodel.Metamodel, scriptExec ScriptExecutor,
	opts ...Option,
) *Workspace {
	var autoEngine *automation.Engine
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
	}

	// Create an empty search index — callers that use a real store
	// populate it from that store. Failures degrade search but don't
	// block construction; Search() surfaces the nil index as an
	// explicit error.
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
		bytesFS:    fs, // default: cleartext passthrough. Overridden in New() when encryption is enabled.
		paths:      paths,
		config:     cfg,
		scriptExec: scriptExec,
		store:      memstore.New(),
		meta:       meta,
		automation: autoEngine,
		searchIdx:  searchIdx,
	}
	for _, opt := range opts {
		opt(ws)
	}
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
// the workspace's authoritative store. In production the store is an
// fsstore which persists to disk; in tests it is an in-memory store.
func (w *Workspace) writeEntity(e *entity.Entity) error {
	if e == nil {
		return nil
	}
	return storeUpsertEntity(w.currentStore(), e)
}

// deleteEntityStore is the write-through delete path. Cascade is false
// because the workspace's CRUD already deletes incident relations
// explicitly before calling this.
func (w *Workspace) deleteEntityStore(id string) error {
	if id == "" {
		return nil
	}
	st := w.currentStore()
	if st == nil {
		return nil
	}
	if _, err := st.DeleteEntity(context.Background(), id, false); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return nil
}

// writeRelation upserts a relation into the authoritative store.
func (w *Workspace) writeRelation(r *entity.Relation) error {
	if r == nil {
		return nil
	}
	return storeUpsertRelation(w.currentStore(), r)
}

// deleteRelationStore removes a relation from the authoritative store.
func (w *Workspace) deleteRelationStore(from, relType, to string) error {
	st := w.currentStore()
	if st == nil {
		return nil
	}
	err := st.DeleteRelation(context.Background(), from, relType, to)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return nil
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

// search runs the legacy word/phrase Bleve query against the workspace's
// search index. External callers use Searcher() instead; this method
// backs that adapter.
func (w *Workspace) search(words, phrases []string, limit int) ([]*entity.Entity, []float64, error) {
	if w.searchIdx == nil {
		return nil, nil, errors.New("search index not available")
	}
	results, err := w.searchIdx.Search(words, phrases, limit)
	if err != nil {
		return nil, nil, err
	}
	ctx := context.Background()
	entities := make([]*entity.Entity, 0, len(results))
	scores := make([]float64, 0, len(results))
	for _, r := range results {
		e, err := w.store.GetEntity(ctx, r.ID)
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
	return nil, errors.New("workspace: no repository configured")
}

type nopState struct{}

func (nopState) Get(context.Context, string) ([]byte, error) {
	return nil, errors.New("workspace: no repository configured")
}

func (nopState) Put(context.Context, string, []byte) error {
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

// --- Lifecycle ---

// startSearchReindex subscribes to the store's event stream and updates
// the search index on every create/update/delete. The returned cancel is
// stored on the workspace and invoked by Close.
func (w *Workspace) startSearchReindex() {
	if w.searchIdx == nil || w.store == nil {
		return
	}
	ch, cancel := w.store.Subscribe(storeEventBufferSize)
	w.stopSearch = cancel
	go w.reindexLoop(ch)
}

func (w *Workspace) reindexLoop(ch <-chan store.Event) {
	ctx := context.Background()
	for ev := range ch {
		switch ev.Op {
		case store.EventEntityCreated, store.EventEntityUpdated:
			if ev.EntityID == "" {
				continue
			}
			e, err := w.store.GetEntity(ctx, ev.EntityID)
			if err != nil {
				continue
			}
			if err := w.searchIdx.Index(entityToSearchDocument(e, w.meta)); err != nil {
				slog.Warn("search index update failed", "id", ev.EntityID, "error", err)
			}
		case store.EventEntityDeleted:
			if ev.EntityID == "" {
				continue
			}
			if err := w.searchIdx.Remove(ev.EntityID); err != nil {
				slog.Warn("search index remove failed", "id", ev.EntityID, "error", err)
			}
		case store.EventRelationCreated, store.EventRelationUpdated, store.EventRelationDeleted:
			// Relations don't affect the entity search index.
		}
	}
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
// defaults, validates, writes to the store, and runs automation.
func (w *Workspace) createEntity(entityType string, opts CreateOptions) (*entity.Entity, *CreateResult, error) {
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
	if w.automation != nil {
		autoResult = w.automation.Process(automation.Event{
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
// and persists to the authoritative store.
func (w *Workspace) updateEntity(entity, oldEntity *entity.Entity) (*UpdateResult, error) {
	// Validate.
	if errs := w.meta.ValidateEntity(entity.ID, entity.Type, entity.Properties); len(errs) > 0 {
		return nil, newValidationError(errs)
	}

	result := &UpdateResult{}

	// Run automation to get property changes and side effects.
	var autoResult *automation.Result
	if w.automation != nil && oldEntity != nil {
		autoResult = w.automation.Process(automation.Event{
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

	// Write to store BEFORE side effects. This ensures Lua scripts can
	// modify entities without being overwritten.
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
var ErrHasRelations = errors.New("entity has relations; set cascade=true to delete")

// DeleteEntity removes an entity and optionally cascades to its relations.
func (w *Workspace) deleteEntity(_, id string, cascade bool) (*DeleteResult, error) {
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

	// Persist to the authoritative store (disk via fsstore in production).
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
		newItem := w.runCreatedEntityAutomation(created, effects)
		if newItem != nil {
			newItems = append(newItems, *newItem)
		}
	}

	return newItems
}

// runCreatedEntityAutomation runs automation on a newly created entity and returns a queue item if needed.
func (w *Workspace) runCreatedEntityAutomation(
	created *entity.Entity,
	effects *automationSideEffects,
) *automationQueueItem {
	if w.automation == nil {
		return nil
	}

	newAutoResult := w.automation.Process(automation.Event{
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
				"automation relation target not found: "+rel.To)
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
	newEntity *entity.Entity,
	oldEntity *entity.Entity,
	luaActions []automation.LuaToExecute,
	effects *automationSideEffects,
) {
	if len(luaActions) == 0 {
		return
	}

	deps := w.LuaWriteDeps()

	for _, action := range luaActions {
		var err error

		switch {
		case action.Code != "":
			err = w.scriptExec.ExecuteCode(action.Code, deps, newEntity, oldEntity)
		case action.FilePath != "":
			err = w.scriptExec.ExecuteFile(action.FilePath, deps, newEntity, oldEntity)
		default:
			// Empty action - skip
			continue
		}

		if err != nil {
			effects.Errors = append(effects.Errors,
				"script execution error: "+err.Error())
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
// validates against the metamodel, and writes to the authoritative store.
func (w *Workspace) createRelation(from, relType, to string, opts ...CreateRelationOptions) (*entity.Relation, error) {
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
	if vErr := w.meta.ValidateRelation(relType, fromEntity.Type, toEntity.Type); vErr != nil {
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
func (w *Workspace) Close() error {
	w.StopWatching()
	if w.stopSearch != nil {
		w.stopSearch()
		w.stopSearch = nil
	}
	if w.searchIdx != nil {
		if err := w.searchIdx.Close(); err != nil {
			w.searchIdx = nil
			return fmt.Errorf("close search index: %w", err)
		}
		w.searchIdx = nil
	}
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
