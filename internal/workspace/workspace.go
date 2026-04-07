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

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/views"
)

// ChangeEvent is re-exported from repository so consumers don't need to
// import repository directly for watcher callback signatures.
type ChangeEvent = repository.ChangeEvent

// ChangeOp is re-exported from repository for the same reason as ChangeEvent.
type ChangeOp = repository.ChangeOp

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
// The workspace field satisfies lua.WorkspaceInterface (verified at runtime
// by the script package).
type scriptContextImpl struct {
	workspace   *Workspace
	meta        *metamodel.Metamodel
	projectRoot string
	entity      *model.Entity
	oldEntity   *model.Entity
}

func (c *scriptContextImpl) GetWorkspace() interface{}     { return c.workspace }
func (c *scriptContextImpl) GetMeta() *metamodel.Metamodel { return c.meta }
func (c *scriptContextImpl) GetProjectRoot() string        { return c.projectRoot }
func (c *scriptContextImpl) GetEntity() *model.Entity      { return c.entity }
func (c *scriptContextImpl) GetOldEntity() *model.Entity   { return c.oldEntity }

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

// Workspace is a stateful domain session that ties together the repository
// (persistence), graph (in-memory query), metamodel (schema), and automation
// engine. All write operations go through Workspace so that disk and memory
// stay in sync.
type Workspace struct {
	repo       repository.Store
	graph      *graph.Graph
	meta       *metamodel.Metamodel
	automation *automation.Engine
	searchIdx  *search.Index
	config     *project.Config
	scriptExec ScriptExecutor
	mu         sync.RWMutex

	// Watcher state (nil when not watching).
	watchHandle *repository.WatchHandle
}

// maxAutomationDepth limits recursive automation triggering. When an entity
// is created by automation, it can trigger further automations up to this
// depth. Beyond this limit, entities are still created but automations are
// skipped with a warning. This prevents infinite loops from misconfigured
// automations while allowing useful chaining (e.g., ticket → checklist → items).
const maxAutomationDepth = 50

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
	repo := repository.New(fs, ctx)
	return New(repo, scriptExec)
}

// New creates a workspace from a repository with a script executor.
// It loads the metamodel, initializes the graph (from cache or by syncing
// from disk), and sets up the automation engine.
//
// For production use, pass script.NewEngine() for Lua support.
// For tests, pass NopScriptExecutor.
func New(repo repository.Store, scriptExec ScriptExecutor) (*Workspace, error) {
	exec := scriptExec
	if exec == nil {
		exec = NopScriptExecutor
	}

	meta, err := repo.LoadMetamodel()
	if err != nil {
		return nil, fmt.Errorf("load metamodel: %w", err)
	}

	g := graph.New()

	// Try cache first, fall back to full sync.
	needsSync := !repo.CacheExists()
	if !needsSync {
		if cacheErr := repo.LoadCache(g); cacheErr != nil {
			if errors.Is(cacheErr, repository.ErrCacheVersionMismatch) {
				slog.Warn("cache outdated, rebuilding", "error", cacheErr)
			}
			needsSync = true
		}
	}
	if needsSync {
		if _, syncErr := repo.Sync(meta, g); syncErr != nil {
			return nil, fmt.Errorf("sync: %w", syncErr)
		}
		// Save the new cache after sync
		if saveErr := repo.SaveCache(g); saveErr != nil {
			slog.Warn("failed to save cache", "error", saveErr)
		}
	}

	return newWorkspace(repo, meta, g, exec), nil
}

// NewWithGraph creates a workspace with a pre-populated graph. Use this
// when the caller has already loaded the metamodel and synced the graph.
//
// The optional scriptExec parameter enables Lua automation actions. If not provided,
// defaults to NopScriptExecutor (suitable for tests). Pass a script.Engine for production.
func NewWithGraph(
	repo repository.Store, meta *metamodel.Metamodel, g *graph.Graph, scriptExec ...ScriptExecutor,
) *Workspace {
	exec := NopScriptExecutor
	if len(scriptExec) > 0 && scriptExec[0] != nil {
		exec = scriptExec[0]
	}
	return newWorkspace(repo, meta, g, exec)
}

// NewForTest creates a minimal workspace for testing. It has no repository,
// so write operations will panic. Use this for unit tests that only need
// to query the graph. It initializes a search index with all entities.
func NewForTest(g *graph.Graph, meta *metamodel.Metamodel) *Workspace {
	ws := &Workspace{
		graph:  g,
		meta:   meta,
		config: project.DefaultConfig(),
	}

	// Initialize search index for test workspaces.
	idx, err := search.NewIndex()
	if err != nil {
		slog.Warn("failed to create test search index", "error", err)
	} else {
		docs := entitiesToSearchDocuments(g.AllNodes(), meta)
		if indexErr := idx.IndexBatch(docs); indexErr != nil {
			slog.Warn("failed to index test entities", "error", indexErr)
		}
		ws.searchIdx = idx
	}

	return ws
}

func newWorkspace(
	repo repository.Store, meta *metamodel.Metamodel, g *graph.Graph, scriptExec ScriptExecutor,
) *Workspace {
	var autoEngine *automation.Engine
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
	}

	// Create search index and index all entities.
	searchIdx, err := search.NewIndex()
	if err != nil {
		slog.Warn("failed to create search index", "error", err)
	} else {
		docs := entitiesToSearchDocuments(g.AllNodes(), meta)
		if err := searchIdx.IndexBatch(docs); err != nil {
			slog.Warn("failed to index entities", "error", err)
		}
	}

	// Load project config (use defaults if not found or repo is nil).
	var cfg *project.Config
	if repo != nil {
		var err error
		cfg, err = project.LoadConfig(repo.Paths())
		if err != nil {
			slog.Warn("failed to load config", "error", err)
			cfg = project.DefaultConfig()
		}
	} else {
		cfg = project.DefaultConfig()
	}

	return &Workspace{
		repo:       repo,
		graph:      g,
		meta:       meta,
		automation: autoEngine,
		searchIdx:  searchIdx,
		config:     cfg,
		scriptExec: scriptExec,
	}
}

// --- Accessors ---

// Graph returns the in-memory graph for direct read queries.
func (w *Workspace) Graph() *graph.Graph { return w.graph }

// Meta returns the current metamodel.
func (w *Workspace) Meta() *metamodel.Metamodel {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.meta
}

// Repo returns the underlying repository for low-level operations not
// wrapped by Workspace (e.g., FS access, Watch).
func (w *Workspace) Repo() repository.Store { return w.repo }

// Search performs a full-text search and returns matching entities with scores.
// words are OR'd together with fuzzy matching; phrases must all match exactly.
func (w *Workspace) Search(words, phrases []string, limit int) ([]*model.Entity, []float64, error) {
	w.mu.RLock()
	idx := w.searchIdx
	w.mu.RUnlock()

	if idx == nil {
		return nil, nil, fmt.Errorf("search index not available")
	}
	results, err := idx.Search(words, phrases, limit)
	if err != nil {
		return nil, nil, err
	}
	entities := make([]*model.Entity, 0, len(results))
	scores := make([]float64, 0, len(results))
	for _, r := range results {
		if e, ok := w.graph.GetNode(r.ID); ok {
			entities = append(entities, e)
			scores = append(scores, r.Score)
		}
	}
	return entities, scores, nil
}

// SearchSimple performs a simple text search (convenience method).
func (w *Workspace) SearchSimple(query string, limit int) ([]*model.Entity, error) {
	entities, _, err := w.Search(strings.Fields(query), nil, limit)
	return entities, err
}

// --- Project accessors ---

// Paths returns the project directory layout.
func (w *Workspace) Paths() *project.Context { return w.repo.Paths() }

// ReadProjectFile reads a file relative to the project root.
func (w *Workspace) ReadProjectFile(name string) ([]byte, error) {
	return w.repo.ReadProjectFile(name)
}

// ReadCacheFile reads a file from the .rela cache directory.
func (w *Workspace) ReadCacheFile(name string) ([]byte, error) {
	return w.repo.ReadCacheFile(name)
}

// WriteCacheFile writes a file to the .rela cache directory.
func (w *Workspace) WriteCacheFile(name string, data []byte) error {
	return w.repo.WriteCacheFile(name, data)
}

// DiscoverEntityTemplates returns all templates (including variants) for an entity type.
func (w *Workspace) DiscoverEntityTemplates(entityType string) ([]*markdown.EntityTemplate, error) {
	return w.repo.DiscoverEntityTemplates(entityType)
}

// GenerateEntityTemplate generates a template file for the given entity type.
func (w *Workspace) GenerateEntityTemplate(entityType, variant string, force bool) (bool, error) {
	return w.repo.GenerateEntityTemplate(w.Meta(), entityType, variant, force)
}

// GenerateRelationTemplate generates a template file for the given relation type.
func (w *Workspace) GenerateRelationTemplate(relationType string, force bool) (bool, error) {
	return w.repo.GenerateRelationTemplate(w.Meta(), relationType, force)
}

// FindOrphanedTempFiles returns paths of leftover .new temp files.
func (w *Workspace) FindOrphanedTempFiles() ([]string, error) {
	return w.repo.FindOrphanedTempFiles()
}

// CleanupOrphanedTempFiles removes leftover .new temp files.
func (w *Workspace) CleanupOrphanedTempFiles() (int, error) {
	return w.repo.CleanupOrphanedTempFiles()
}

// --- Lifecycle ---

// Sync clears the graph and reloads all entities and relations from disk.
func (w *Workspace) Sync() (*model.SyncResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.repo.Sync(w.meta, w.graph)
}

// SyncLua is a Lua-friendly wrapper for Sync that doesn't return the result.
// This satisfies lua.WorkspaceInterface without importing result types.
func (w *Workspace) SyncLua() error {
	_, err := w.Sync()
	return err
}

// Reload reloads the metamodel and re-syncs the graph from disk. This is
// called automatically by the file watcher but is also available for
// programmatic use (after migration, in tests, etc.).
func (w *Workspace) Reload() (*model.SyncResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reloadLocked()
}

func (w *Workspace) reloadLocked() (*model.SyncResult, error) {
	newMeta, err := w.repo.LoadMetamodel()
	if err != nil {
		if migration.IsMigrationError(err) {
			slog.Warn("metamodel needs migration, skipping reload: run 'rela migrate'")
			// Sync with current meta even if metamodel file changed.
			return w.repo.Sync(w.meta, w.graph)
		}
		return nil, fmt.Errorf("reload metamodel: %w", err)
	}
	w.meta = newMeta

	// Rebuild automation engine for new metamodel.
	if len(newMeta.Automations) > 0 {
		w.automation = automation.NewEngineFromMetamodel(newMeta.Automations)
	} else {
		w.automation = nil
	}

	result, err := w.repo.Sync(w.meta, w.graph)
	if err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}

	// Rebuild search index.
	w.rebuildSearchIndex()

	w.saveCacheQuietly()
	return result, nil
}

// rebuildSearchIndex recreates and repopulates the search index.
// Caller must hold w.mu.Lock.
func (w *Workspace) rebuildSearchIndex() {
	if w.searchIdx != nil {
		if err := w.searchIdx.Close(); err != nil {
			slog.Warn("failed to close search index", "error", err)
		}
	}
	idx, err := search.NewIndex()
	if err != nil {
		slog.Warn("failed to create search index", "error", err)
		w.searchIdx = nil
		return
	}
	docs := entitiesToSearchDocuments(w.graph.AllNodes(), w.meta)
	if err := idx.IndexBatch(docs); err != nil {
		slog.Warn("failed to index entities", "error", err)
	}
	w.searchIdx = idx
}

// indexEntity adds or updates an entity in the search index.
func (w *Workspace) indexEntity(entity *model.Entity) {
	w.mu.RLock()
	idx := w.searchIdx
	meta := w.meta
	w.mu.RUnlock()

	if idx != nil {
		doc := entityToSearchDocument(entity, meta)
		if err := idx.Index(doc); err != nil {
			slog.Warn("failed to index entity", "id", entity.ID, "error", err)
		}
	}
}

// removeFromIndex removes an entity from the search index.
func (w *Workspace) removeFromIndex(id string) {
	w.mu.RLock()
	idx := w.searchIdx
	w.mu.RUnlock()

	if idx != nil {
		if err := idx.Remove(id); err != nil {
			slog.Warn("failed to remove entity from index", "id", id, "error", err)
		}
	}
}

// SaveCache persists the graph to the cache file.
func (w *Workspace) SaveCache() error {
	return w.repo.SaveCache(w.graph)
}

func (w *Workspace) saveCacheQuietly() {
	if err := w.repo.SaveCache(w.graph); err != nil {
		slog.Warn("failed to save cache", "error", err)
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
	meta := w.Meta()
	entityDef, ok := meta.GetEntityDef(entityType)
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

	existingIDs := w.graph.AllIDs()
	if entityDef.IsShortID() {
		return model.GenerateShortID(existingIDs, prefix, w.graph.NodeCount(), entityDef.GetIDCaps()), nil
	}
	return model.GenerateNextID(existingIDs, prefix), nil
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
	RelationsCreated   []*model.Relation
	EntitiesCreated    []*model.Entity
}

// CreateEntity generates an ID (unless provided), applies templates and
// defaults, validates, writes to disk, updates the graph, and runs
// automation.
func (w *Workspace) CreateEntity(entityType string, opts CreateOptions) (*model.Entity, *CreateResult, error) {
	// Check for duplicates if custom ID provided.
	if opts.ID != "" {
		if _, exists := w.graph.GetNode(opts.ID); exists {
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
			if err := w.repo.WriteEntity(entity, w.Meta()); err != nil {
				return nil, nil, fmt.Errorf("write entity after automation: %w", err)
			}
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors
	}

	// Index the entity for search.
	w.indexEntity(entity)

	// Apply automation side effects (relations, entities, Lua) after entity is written.
	if autoResult != nil {
		effects := w.applyAutomationSideEffects(entity, nil, autoResult)
		result.RelationsCreated = effects.RelationsCreated
		result.EntitiesCreated = effects.EntitiesCreated
		result.AutomationErrors = append(result.AutomationErrors, effects.Errors...)
		result.AutomationWarnings = append(result.AutomationWarnings, effects.Warnings...)
	}

	w.saveCacheQuietly()
	return entity, result, nil
}

// UpdateResult contains side-effects from entity update.
type UpdateResult struct {
	AutomationWarnings []string
	AutomationErrors   []string
	RelationsCreated   []*model.Relation
	EntitiesCreated    []*model.Entity
}

// UpdateEntity validates and writes an existing entity, runs automation,
// and updates the graph.
func (w *Workspace) UpdateEntity(entity, oldEntity *model.Entity) (*UpdateResult, error) {
	meta := w.Meta()

	// Validate.
	if errs := meta.ValidateEntity(entity); len(errs) > 0 {
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

	// Write to disk + update graph + search index BEFORE side effects.
	// This ensures Lua scripts can modify entities without being overwritten.
	if err := w.repo.WriteEntity(entity, meta); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}
	w.graph.AddNode(entity)
	w.indexEntity(entity)

	// Apply automation side effects (relations, entities, Lua) AFTER entity is written.
	if autoResult != nil {
		effects := w.applyAutomationSideEffects(entity, oldEntity, autoResult)
		result.RelationsCreated = effects.RelationsCreated
		result.EntitiesCreated = effects.EntitiesCreated
		result.AutomationErrors = append(result.AutomationErrors, effects.Errors...)
		result.AutomationWarnings = append(result.AutomationWarnings, effects.Warnings...)
	}

	w.saveCacheQuietly()
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
func (w *Workspace) DeleteEntity(entityType, id string, cascade bool) (*DeleteResult, error) {
	if _, ok := w.graph.GetNode(id); !ok {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	incoming := w.graph.IncomingEdges(id)
	outgoing := w.graph.OutgoingEdges(id)
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return nil, ErrHasRelations
	}

	result := &DeleteResult{}
	meta := w.Meta()

	// Delete relations first.
	for _, rel := range incoming {
		if err := w.repo.DeleteRelation(rel.From, rel.Type, rel.To); err != nil {
			slog.Warn("failed to delete relation", "from", rel.From, "type", rel.Type, "to", rel.To, "error", err)
		}
		w.graph.RemoveEdge(rel.From, rel.Type, rel.To)
		result.RelationsDeleted++
	}
	for _, rel := range outgoing {
		if err := w.repo.DeleteRelation(rel.From, rel.Type, rel.To); err != nil {
			slog.Warn("failed to delete relation", "from", rel.From, "type", rel.Type, "to", rel.To, "error", err)
		}
		w.graph.RemoveEdge(rel.From, rel.Type, rel.To)
		result.RelationsDeleted++
	}

	// Delete entity.
	if err := w.repo.DeleteEntity(entityType, id, meta); err != nil {
		return nil, fmt.Errorf("delete entity: %w", err)
	}
	w.graph.RemoveNode(id)
	w.removeFromIndex(id)

	w.saveCacheQuietly()
	return result, nil
}

// --- Lua-specific interface methods ---
// These methods satisfy lua.WorkspaceInterface without importing result types,
// breaking the circular dependency between lua and workspace packages.

// CreateEntityLua creates an entity and returns it without the result struct.
// This satisfies lua.WorkspaceInterface using primitive types to avoid import cycles.
func (w *Workspace) CreateEntityLua(
	entityType, id string, props map[string]interface{}, content string,
) (*model.Entity, error) {
	entity, _, err := w.CreateEntity(entityType, CreateOptions{
		ID:         id,
		Properties: props,
		Content:    content,
	})
	return entity, err
}

// UpdateEntityLua updates an entity without returning the result struct.
// This satisfies lua.WorkspaceInterface.
func (w *Workspace) UpdateEntityLua(entity, oldEntity *model.Entity) error {
	_, err := w.UpdateEntity(entity, oldEntity)
	return err
}

// DeleteEntityLua deletes an entity without returning the result struct.
// This satisfies lua.WorkspaceInterface.
func (w *Workspace) DeleteEntityLua(entityType, id string, cascade bool) error {
	_, err := w.DeleteEntity(entityType, id, cascade)
	return err
}

// CreateRelationLua creates a relation without options.
// This satisfies lua.WorkspaceInterface.
func (w *Workspace) CreateRelationLua(from, relType, to string) (*model.Relation, error) {
	return w.CreateRelation(from, relType, to)
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
func (w *Workspace) createEntityCore(entityType string, opts createEntityCoreOpts) (*model.Entity, error) {
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
		if err := model.ValidateID(entityID); err != nil {
			return nil, err
		}
	}

	entity := model.NewEntity(entityID, entityType)

	// Apply template defaults (use variant if specified).
	template, err := w.repo.LoadEntityTemplateVariant(entityType, opts.TemplateVariant)
	if err != nil {
		return nil, fmt.Errorf("load template: %w", err)
	}
	// If a variant was explicitly specified but not found, that's an error.
	if opts.TemplateVariant != "" && template == nil {
		return nil, fmt.Errorf("template variant %q not found for entity type %s", opts.TemplateVariant, entityType)
	}
	if template != nil {
		markdown.ApplyEntityTemplate(entity, template)
	}

	// Apply provided properties (override template defaults).
	for k, v := range opts.Properties {
		entity.Properties[k] = v
	}

	// Set body content.
	if opts.Content != "" {
		entity.Content = opts.Content
	}

	// Set default status if not set.
	if entity.GetString("status") == "" {
		entity.SetString("status", entityDef.GetDefaultStatus(meta))
	}

	// Validate.
	if errs := meta.ValidateEntity(entity); len(errs) > 0 {
		return nil, newValidationError(errs)
	}

	// Write to disk + update graph.
	if err := w.repo.WriteEntity(entity, meta); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}
	w.graph.AddNode(entity)

	return entity, nil
}

// automationSideEffects holds entities and relations created by automation.
type automationSideEffects struct {
	RelationsCreated []*model.Relation
	EntitiesCreated  []*model.Entity
	Errors           []string
	Warnings         []string
}

// findExistingRelationTarget finds an existing entity of the given type that is
// the target of a relation from the source entity with the given relation type.
// Returns nil if no such entity exists.
func (w *Workspace) findExistingRelationTarget(sourceID, relationType, targetType string) *model.Entity {
	for _, rel := range w.graph.OutgoingEdges(sourceID) {
		if rel.Type == relationType {
			if target, ok := w.graph.GetNode(rel.To); ok && target.Type == targetType {
				return target
			}
		}
	}
	return nil
}

// automationQueueItem represents a pending automation result to process.
type automationQueueItem struct {
	trigger    *model.Entity
	autoResult *automation.Result
}

// applyAutomationSideEffects processes automation results iteratively using a BFS queue.
// This avoids deep recursion and provides clear iteration limits.
func (w *Workspace) applyAutomationSideEffects(
	triggerEntity *model.Entity,
	oldEntity *model.Entity,
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
	trigger *model.Entity,
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
	created *model.Entity,
	meta *metamodel.Metamodel,
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
		if err := w.repo.WriteEntity(created, meta); err != nil {
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
	triggerEntity *model.Entity,
	relations []*model.Relation,
	effects *automationSideEffects,
) {
	meta := w.Meta()

	for _, rel := range relations {
		rel.From = triggerEntity.ID

		targetEntity, ok := w.graph.GetNode(rel.To)
		if !ok {
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
	entity *model.Entity,
	oldEntity *model.Entity,
	luaActions []automation.LuaToExecute,
	effects *automationSideEffects,
) {
	if len(luaActions) == 0 {
		return
	}

	// Build script context once for all actions
	ctx := &scriptContextImpl{
		workspace:   w,
		meta:        w.meta,
		projectRoot: w.repo.Paths().Root,
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
	triggerEntity *model.Entity,
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
		if _, err := w.DeleteEntity(existingTarget.Type, existingTarget.ID, true); err != nil {
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
	triggerEntity, created *model.Entity,
	relationType string,
	effects *automationSideEffects,
) {
	meta := w.Meta()

	if err := meta.ValidateRelation(relationType, triggerEntity.Type, created.Type); err != nil {
		effects.Errors = append(effects.Errors,
			fmt.Sprintf("automation relation invalid: %v", err))
		return
	}

	rel := model.NewRelation(triggerEntity.ID, relationType, created.ID)
	if err := w.writeRelationCore(rel); err != nil {
		effects.Errors = append(effects.Errors,
			fmt.Sprintf("failed to create automation relation: %v", err))
		return
	}
	effects.RelationsCreated = append(effects.RelationsCreated, rel)
}

// --- Relation operations ---

// writeRelationCore writes a relation to disk and updates the graph.
// This is the shared write logic used by CreateRelation and automation processing.
func (w *Workspace) writeRelationCore(rel *model.Relation) error {
	if err := w.repo.WriteRelation(rel); err != nil {
		return fmt.Errorf("write relation: %w", err)
	}
	w.graph.AddEdge(rel)
	return nil
}

// CreateRelationOptions configures optional settings for relation creation.
type CreateRelationOptions struct {
	Properties map[string]interface{} // property values for the relation
	Content    string                 // markdown body content for the relation
}

// CreateRelation validates both endpoints exist, checks for duplicates,
// validates against the metamodel, writes to disk, and updates the graph.
func (w *Workspace) CreateRelation(from, relType, to string, opts ...CreateRelationOptions) (*model.Relation, error) {
	meta := w.Meta()

	fromEntity, ok := w.graph.GetNode(from)
	if !ok {
		return nil, fmt.Errorf("source entity not found: %s", from)
	}
	toEntity, ok := w.graph.GetNode(to)
	if !ok {
		return nil, fmt.Errorf("target entity not found: %s", to)
	}

	// Validate relation type.
	if err := meta.ValidateRelation(relType, fromEntity.Type, toEntity.Type); err != nil {
		return nil, fmt.Errorf("invalid relation: %w", err)
	}

	// Check for duplicates.
	if _, exists := w.graph.GetEdge(from, relType, to); exists {
		return nil, fmt.Errorf("relation already exists: %s --%s--> %s", from, relType, to)
	}

	rel := model.NewRelation(from, relType, to)

	// Apply template if available.
	template, err := w.repo.LoadRelationTemplate(relType)
	if err != nil {
		return nil, fmt.Errorf("load relation template: %w", err)
	}
	if template != nil {
		markdown.ApplyRelationTemplate(rel, template)
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

	w.saveCacheQuietly()
	return rel, nil
}

// UpdateRelation updates properties on an existing relation.
func (w *Workspace) UpdateRelation(from, relType, to string, opts CreateRelationOptions) (*model.Relation, error) {
	rel, exists := w.graph.GetEdge(from, relType, to)
	if !exists {
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

	w.saveCacheQuietly()
	return rel, nil
}

// DeleteRelation removes a relation from disk and the graph.
func (w *Workspace) DeleteRelation(from, relType, to string) error {
	if err := w.repo.DeleteRelation(from, relType, to); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	w.graph.RemoveEdge(from, relType, to)
	w.saveCacheQuietly()
	return nil
}

// --- Rename ---

// RenameEntity renames an entity, updating all references in relations.
func (w *Workspace) RenameEntity(entityType, oldID, newID string, dryRun bool) (*rename.Result, error) {
	return rename.Rename(w.repo, w.Meta(), w.graph, entityType, oldID, newID, rename.Options{DryRun: dryRun})
}

// --- Formatting ---

// FormatEntity checks if an entity file needs formatting and optionally writes
// the formatted version. Returns true if the file was (or would be) modified.
func (w *Workspace) FormatEntity(entity *model.Entity, dryRun bool) (bool, error) {
	// Get property order from metamodel
	var propertyOrder []string
	if entityDef, ok := w.meta.GetEntityDef(entity.Type); ok {
		propertyOrder = entityDef.GetPropertyOrder()
	}

	// Generate formatted content with configured line width
	formatted, err := markdown.FormatEntityWithWidth(entity, propertyOrder, w.config.Formatting.LineWidth)
	if err != nil {
		return false, fmt.Errorf("format entity: %w", err)
	}

	// Read current file content
	current, err := w.repo.FS().ReadFile(entity.FilePath)
	if err != nil {
		return false, fmt.Errorf("read entity file: %w", err)
	}

	// Compare
	if formatted == string(current) {
		return false, nil
	}

	// Write if not dry-run
	if !dryRun {
		if err := w.repo.WriteEntity(entity, w.meta); err != nil {
			return false, fmt.Errorf("write entity: %w", err)
		}
	}

	return true, nil
}

// FormatRelation checks if a relation file needs formatting and optionally writes
// the formatted version. Returns true if the file was (or would be) modified.
func (w *Workspace) FormatRelation(relation *model.Relation, dryRun bool) (bool, error) {
	// Generate formatted content with configured line width
	formatted, err := markdown.FormatRelationWithWidth(relation, w.config.Formatting.LineWidth)
	if err != nil {
		return false, fmt.Errorf("format relation: %w", err)
	}

	// Read current file content
	current, err := w.repo.FS().ReadFile(relation.FilePath)
	if err != nil {
		return false, fmt.Errorf("read relation file: %w", err)
	}

	// Compare
	if formatted == string(current) {
		return false, nil
	}

	// Write if not dry-run
	if !dryRun {
		if err := w.repo.WriteRelation(relation); err != nil {
			return false, fmt.Errorf("write relation: %w", err)
		}
	}

	return true, nil
}

// --- File watching ---

// WatchOptions configures the file watcher.
type WatchOptions struct {
	// ExtraFiles lists additional files to watch (e.g., data-entry.yaml).
	ExtraFiles []string
	// ExtraDirs lists additional directories to watch (e.g., metamodel/).
	ExtraDirs []string
	// OnReload is called after workspace has reloaded metamodel and graph.
	// Consumers use this for side-effects (SSE broadcast, MCP notifications, etc.).
	OnReload func(events []ChangeEvent)
}

// StartWatching begins watching for file changes. On each change the
// workspace reloads the metamodel, re-syncs the graph, saves the cache,
// and then calls OnReload.
func (w *Workspace) StartWatching(opts WatchOptions) error {
	repoOpts := repository.WatchOptions{
		ExtraFiles: opts.ExtraFiles,
		ExtraDirs:  opts.ExtraDirs,
	}
	handle, err := w.repo.WatchWithHandle(repoOpts, func(events []repository.ChangeEvent) {
		w.mu.Lock()
		_, reloadErr := w.reloadLocked()
		w.mu.Unlock()

		if reloadErr != nil {
			slog.Error("reload error", "error", reloadErr)
		}
		if opts.OnReload != nil {
			opts.OnReload(events)
		}
	})
	if err != nil {
		return err
	}
	w.watchHandle = handle
	return nil
}

// StopWatching stops the file watcher.
func (w *Workspace) StopWatching() {
	if w.watchHandle != nil {
		w.watchHandle.Stop()
		w.watchHandle = nil
	}
}

// Close releases resources held by the workspace (search index, watcher).
func (w *Workspace) Close() error {
	w.StopWatching()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.searchIdx != nil {
		if err := w.searchIdx.Close(); err != nil {
			return fmt.Errorf("close search index: %w", err)
		}
		w.searchIdx = nil
	}
	return nil
}

// PauseWatching temporarily suppresses file change events.
func (w *Workspace) PauseWatching() {
	if w.watchHandle != nil {
		w.watchHandle.Pause()
	}
}

// ResumeWatching re-enables file change events after PauseWatching.
func (w *Workspace) ResumeWatching() {
	if w.watchHandle != nil {
		w.watchHandle.Resume()
	}
}

// --- Locking for consumers ---

// RLock acquires a read lock on the workspace. Consumers that need
// consistent reads across multiple graph queries (e.g., HTTP handlers)
// should hold this lock for the duration of the request.
func (w *Workspace) RLock()   { w.mu.RLock() }
func (w *Workspace) RUnlock() { w.mu.RUnlock() }

// --- Views ---

// LoadViews loads and parses the views.yaml file from the project root.
func (w *Workspace) LoadViews() (*views.File, error) {
	return w.repo.LoadViews()
}

// ExecuteView executes a named view and returns the result.
// Returns an error if views.yaml cannot be loaded or the view doesn't exist.
func (w *Workspace) ExecuteView(viewName, entryID string) (*views.ViewResult, error) {
	viewsFile, err := w.LoadViews()
	if err != nil {
		return nil, fmt.Errorf("loading views.yaml: %w", err)
	}

	viewDef, ok := viewsFile.Views[viewName]
	if !ok {
		return nil, fmt.Errorf("view %q not found in views.yaml", viewName)
	}

	engine := views.NewEngine(w.graph, w.meta)
	return engine.Execute(viewDef, entryID)
}

// --- Filesystem access ---

// FS returns the underlying filesystem for operations that need direct
// file access (e.g., attachment store, writing output files).
func (w *Workspace) FS() storage.FS {
	return w.repo.FS()
}

// --- Search document conversion ---

// entityToSearchDocument converts an entity to a search.Document.
func entityToSearchDocument(e *model.Entity, meta *metamodel.Metamodel) search.Document {
	return search.Document{
		ID:          e.ID,
		Type:        e.Type,
		Primary:     meta.DisplayTitle(e),
		Description: e.Description(),
		Content:     e.Content,
		Properties:  flattenProperties(e.Properties),
	}
}

// entitiesToSearchDocuments converts a slice of entities to search documents.
func entitiesToSearchDocuments(entities []*model.Entity, meta *metamodel.Metamodel) []search.Document {
	docs := make([]search.Document, len(entities))
	for i, e := range entities {
		docs[i] = entityToSearchDocument(e, meta)
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
