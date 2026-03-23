// Package workspace provides a stateful domain session that owns the
// repository, graph, metamodel, and automation engine. It provides
// write-through operations that keep disk and in-memory state in sync,
// eliminating the dual-write pattern that consumers would otherwise
// duplicate.
package workspace

import (
	"errors"
	"fmt"
	"log"
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

// DiscoverAndNew discovers a project from the given start directory and
// creates a workspace. If startDir is empty, it uses the current working
// directory. This is a convenience function that combines project discovery,
// repository creation, and workspace initialization.
func DiscoverAndNew(startDir string) (*Workspace, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, err
	}
	repo := repository.New(fs, ctx)
	return New(repo)
}

// New creates a workspace from a repository. It loads the metamodel,
// initializes the graph (from cache or by syncing from disk), and sets
// up the automation engine.
func New(repo repository.Store) (*Workspace, error) {
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
				log.Printf("Cache outdated, rebuilding: %v", cacheErr)
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
			log.Printf("Warning: failed to save cache: %v", saveErr)
		}
	}

	return newWorkspace(repo, meta, g), nil
}

// NewWithGraph creates a workspace with a pre-populated graph. Use this
// when the caller has already loaded the metamodel and synced the graph.
func NewWithGraph(repo repository.Store, meta *metamodel.Metamodel, g *graph.Graph) *Workspace {
	return newWorkspace(repo, meta, g)
}

// NewForTest creates a minimal workspace for testing. It has no repository,
// so write operations will panic. Use this for unit tests that only need
// to query the graph. It initializes a search index with all entities.
func NewForTest(g *graph.Graph, meta *metamodel.Metamodel) *Workspace {
	ws := &Workspace{
		graph: g,
		meta:  meta,
	}

	// Initialize search index for test workspaces.
	idx, err := search.NewIndex()
	if err != nil {
		log.Printf("Warning: failed to create test search index: %v", err)
	} else {
		docs := entitiesToSearchDocuments(g.AllNodes(), meta)
		if indexErr := idx.IndexBatch(docs); indexErr != nil {
			log.Printf("Warning: failed to index test entities: %v", indexErr)
		}
		ws.searchIdx = idx
	}

	return ws
}

func newWorkspace(repo repository.Store, meta *metamodel.Metamodel, g *graph.Graph) *Workspace {
	var autoEngine *automation.Engine
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
	}

	// Create search index and index all entities.
	searchIdx, err := search.NewIndex()
	if err != nil {
		log.Printf("Warning: failed to create search index: %v", err)
	} else {
		docs := entitiesToSearchDocuments(g.AllNodes(), meta)
		if err := searchIdx.IndexBatch(docs); err != nil {
			log.Printf("Warning: failed to index entities: %v", err)
		}
	}

	return &Workspace{
		repo:       repo,
		graph:      g,
		meta:       meta,
		automation: autoEngine,
		searchIdx:  searchIdx,
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
			log.Printf("Metamodel needs migration, skipping reload: run 'rela migrate'")
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
			log.Printf("Warning: failed to close search index: %v", err)
		}
	}
	idx, err := search.NewIndex()
	if err != nil {
		log.Printf("Warning: failed to create search index: %v", err)
		w.searchIdx = nil
		return
	}
	docs := entitiesToSearchDocuments(w.graph.AllNodes(), w.meta)
	if err := idx.IndexBatch(docs); err != nil {
		log.Printf("Warning: failed to index entities: %v", err)
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
			log.Printf("Warning: failed to index entity %s: %v", entity.ID, err)
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
			log.Printf("Warning: failed to remove entity %s from index: %v", id, err)
		}
	}
}

// SaveCache persists the graph to the cache file.
func (w *Workspace) SaveCache() error {
	return w.repo.SaveCache(w.graph)
}

func (w *Workspace) saveCacheQuietly() {
	if err := w.repo.SaveCache(w.graph); err != nil {
		log.Printf("Warning: failed to save cache: %v", err)
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
		return model.GenerateShortID(existingIDs, prefix, w.graph.NodeCount()), nil
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
	meta := w.Meta()
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil, nil, fmt.Errorf("unknown entity type: %s", entityType)
	}

	// Resolve ID.
	entityID := opts.ID
	if entityID == "" {
		id, err := w.GenerateID(entityType, opts.Prefix)
		if err != nil {
			return nil, nil, err
		}
		entityID = id
	} else {
		if err := model.ValidateID(entityID); err != nil {
			return nil, nil, err
		}
	}

	// Check for duplicates.
	if _, exists := w.graph.GetNode(entityID); exists {
		return nil, nil, fmt.Errorf("entity with ID %s already exists", entityID)
	}

	entity := model.NewEntity(entityID, entityType)

	// Apply template defaults.
	template, err := w.repo.LoadEntityTemplate(entityType)
	if err != nil {
		return nil, nil, fmt.Errorf("load template: %w", err)
	}
	if template != nil {
		markdown.ApplyEntityTemplate(entity, template)
	}

	// Apply caller-provided properties (override template defaults).
	for k, v := range opts.Properties {
		entity.Properties[k] = v
	}

	// Set body.
	if opts.Content != "" {
		entity.Content = opts.Content
	}

	// Set default status if not set.
	if entity.GetString("status") == "" {
		entity.SetString("status", entityDef.GetDefaultStatus(meta))
	}

	// Run automation once and collect all results.
	result := &CreateResult{}
	var autoResult *automation.Result
	if w.automation != nil {
		autoResult = w.automation.Process(automation.Event{
			Type:   automation.EventEntityCreated,
			Entity: entity,
		})
		// Apply property changes before validation.
		for prop, val := range autoResult.PropertiesSet {
			entity.SetString(prop, val)
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors
	}

	// Validate.
	if errs := meta.ValidateEntity(entity); len(errs) > 0 {
		return nil, nil, newValidationError(errs)
	}

	// Write to disk + update graph + search index.
	if err := w.repo.WriteEntity(entity, meta); err != nil {
		return nil, nil, fmt.Errorf("write entity: %w", err)
	}
	w.graph.AddNode(entity)
	w.indexEntity(entity)

	// Apply automation side effects (relations, entities) after entity is written.
	if autoResult != nil {
		effects := w.applyAutomationSideEffects(entity, autoResult)
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

	// Run automation.
	if w.automation != nil && oldEntity != nil {
		autoResult := w.automation.Process(automation.Event{
			Type:      automation.EventEntityUpdated,
			Entity:    entity,
			OldEntity: oldEntity,
		})
		for prop, val := range autoResult.PropertiesSet {
			entity.SetString(prop, val)
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors

		effects := w.applyAutomationSideEffects(entity, autoResult)
		result.RelationsCreated = effects.RelationsCreated
		result.EntitiesCreated = effects.EntitiesCreated
		result.AutomationErrors = append(result.AutomationErrors, effects.Errors...)
		result.AutomationWarnings = append(result.AutomationWarnings, effects.Warnings...)
	}

	// Write to disk + update graph + search index.
	if err := w.repo.WriteEntity(entity, meta); err != nil {
		return nil, fmt.Errorf("write entity: %w", err)
	}
	w.graph.AddNode(entity)
	w.indexEntity(entity)

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
			log.Printf("Warning: failed to delete relation %s--%s-->%s: %v", rel.From, rel.Type, rel.To, err)
		}
		w.graph.RemoveEdge(rel.From, rel.Type, rel.To)
		result.RelationsDeleted++
	}
	for _, rel := range outgoing {
		if err := w.repo.DeleteRelation(rel.From, rel.Type, rel.To); err != nil {
			log.Printf("Warning: failed to delete relation %s--%s-->%s: %v", rel.From, rel.Type, rel.To, err)
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

// createEntityCore creates an entity without running automations.
// If templateVariant is non-empty, loads <type>--<variant>.md instead of <type>.md.
// This is the core creation logic used by the automation queue processor.
func (w *Workspace) createEntityCore(
	entityType, templateVariant string,
	props map[string]interface{},
) (*model.Entity, error) {
	meta := w.Meta()
	entityDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}

	// Generate ID.
	entityID, err := w.GenerateID(entityType, "")
	if err != nil {
		return nil, err
	}

	entity := model.NewEntity(entityID, entityType)

	// Apply template defaults (use variant if specified).
	template, err := w.repo.LoadEntityTemplateVariant(entityType, templateVariant)
	if err != nil {
		return nil, fmt.Errorf("load template: %w", err)
	}
	// If a variant was explicitly specified but not found, that's an error.
	if templateVariant != "" && template == nil {
		return nil, fmt.Errorf("template variant %q not found for entity type %s", templateVariant, entityType)
	}
	if template != nil {
		markdown.ApplyEntityTemplate(entity, template)
	}

	// Apply provided properties (override template defaults).
	for k, v := range props {
		entity.Properties[k] = v
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
	autoResult *automation.Result,
) *automationSideEffects {
	effects := &automationSideEffects{}
	meta := w.Meta()

	// BFS queue of pending automation results to process.
	queue := []automationQueueItem{{triggerEntity, autoResult}}
	iterations := 0

	for len(queue) > 0 && iterations < maxAutomationDepth {
		// Pop from front (BFS order - process all items at depth N before depth N+1).
		item := queue[0]
		queue = queue[1:]
		iterations++

		// Process relations for this trigger.
		w.applyRelationCreations(item.trigger, item.autoResult.RelationsToCreate, effects)

		// Collect warnings/errors from this automation result.
		effects.Warnings = append(effects.Warnings, item.autoResult.Warnings...)
		effects.Errors = append(effects.Errors, item.autoResult.Errors...)

		// Process entity creations.
		for _, toCreate := range item.autoResult.EntitiesToCreate {
			if skip := w.handleIfExists(item.trigger, toCreate, effects); skip {
				continue
			}

			// Create entity (no automation yet).
			created, createErr := w.createEntityCore(toCreate.Type, toCreate.Template, toCreate.Properties)
			if createErr != nil {
				effects.Errors = append(effects.Errors,
					fmt.Sprintf("failed to create automation entity %s: %v", toCreate.Type, createErr))
				continue
			}
			effects.EntitiesCreated = append(effects.EntitiesCreated, created)

			// Create relation from trigger if specified.
			if toCreate.RelationFromTrigger != "" {
				w.createTriggerRelation(item.trigger, created, toCreate.RelationFromTrigger, effects)
			}

			// Run automation on newly created entity and queue results.
			if w.automation != nil {
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

				// Queue for processing if there's more work to do.
				if len(newAutoResult.EntitiesToCreate) > 0 || len(newAutoResult.RelationsToCreate) > 0 ||
					len(newAutoResult.Warnings) > 0 || len(newAutoResult.Errors) > 0 {
					queue = append(queue, automationQueueItem{created, newAutoResult})
				}
			}
		}
	}

	// Warn if we hit the limit with work remaining.
	if len(queue) > 0 {
		effects.Warnings = append(effects.Warnings,
			fmt.Sprintf("automation iteration limit (%d) reached; %d pending items skipped",
				maxAutomationDepth, len(queue)))
	}

	return effects
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

		if writeErr := w.repo.WriteRelation(rel); writeErr != nil {
			effects.Errors = append(effects.Errors,
				fmt.Sprintf("failed to create automation relation: %v", writeErr))
			continue
		}
		w.graph.AddEdge(rel)
		effects.RelationsCreated = append(effects.RelationsCreated, rel)
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
	if writeErr := w.repo.WriteRelation(rel); writeErr != nil {
		effects.Errors = append(effects.Errors,
			fmt.Sprintf("failed to create automation relation: %v", writeErr))
		return
	}
	w.graph.AddEdge(rel)
	effects.RelationsCreated = append(effects.RelationsCreated, rel)
}

// --- Relation operations ---

// CreateRelationOptions configures optional settings for relation creation.
type CreateRelationOptions struct {
	Properties map[string]interface{} // property values for the relation
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

	// Apply caller-provided properties (override template defaults).
	if len(opts) > 0 {
		for k, v := range opts[0].Properties {
			rel.Properties[k] = v
		}
	}

	if err := w.repo.WriteRelation(rel); err != nil {
		return nil, fmt.Errorf("write relation: %w", err)
	}
	w.graph.AddEdge(rel)

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
			log.Printf("Reload error: %v", reloadErr)
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
