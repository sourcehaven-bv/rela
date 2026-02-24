// Package repository provides a high-level domain API for entity, relation,
// cache, metamodel, and template persistence. It combines storage.FS,
// markdown.FileIO, and project.Context into a single interface, abstracting
// away path computation and file format details.
//
// Note: Repository lives in its own package (not in storage) to avoid
// circular imports — markdown, graph, metamodel, and project all import
// storage for the FS interface.
package repository

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/views"
)

// Store defines the domain-level persistence contract for entities,
// relations, cache, metamodel, views, and templates. Implementations may
// be backed by markdown files on disk, an SQLite database, a remote API,
// or any other storage mechanism.
type Store interface {
	// Paths returns the project context (root directory, entity/relation dirs, etc.).
	Paths() *project.Context

	// --- Entity CRUD ---

	ReadEntity(entityType, id string, meta *metamodel.Metamodel) (*model.Entity, error)
	WriteEntity(entity *model.Entity, meta *metamodel.Metamodel) error
	DeleteEntity(entityType, id string, meta *metamodel.Metamodel) error
	ListEntities(meta *metamodel.Metamodel) ([]*model.Entity, error)

	// --- Relation CRUD ---

	ReadRelation(from, relType, to string) (*model.Relation, error)
	WriteRelation(relation *model.Relation) error
	DeleteRelation(from, relType, to string) error
	ListRelations() ([]*model.Relation, error)

	// --- Sync ---

	Sync(meta *metamodel.Metamodel, g *graph.Graph) (*model.SyncResult, error)

	// --- Cache ---

	SaveCache(g *graph.Graph) error
	LoadCache(g *graph.Graph) error
	CacheExists() bool

	// --- Metamodel ---

	LoadMetamodel() (*metamodel.Metamodel, error)

	// --- Views ---

	LoadViews() (*views.File, error)

	// --- Project Files ---

	// ReadProjectFile reads a file relative to the project root.
	// This allows consumers to read app-specific config files (e.g. data-entry.yaml)
	// without needing direct filesystem access.
	ReadProjectFile(filename string) ([]byte, error)

	// WriteProjectFile writes a file relative to the project root.
	WriteProjectFile(filename string, data []byte) error

	// ReadCacheFile reads a file relative to the cache directory (.rela/).
	// This is for app-specific state files (e.g. ui-state.json).
	ReadCacheFile(filename string) ([]byte, error)

	// WriteCacheFile writes a file relative to the cache directory (.rela/).
	// Creates the cache directory if it doesn't exist.
	WriteCacheFile(filename string, data []byte) error

	// --- Templates ---

	LoadEntityTemplate(entityType string) (*markdown.Document, error)
	LoadRelationTemplate(relationType string) (*markdown.Document, error)
	GenerateEntityTemplate(meta *metamodel.Metamodel, entityType, variant string, force bool) (bool, error)
	GenerateRelationTemplate(meta *metamodel.Metamodel, relationType string, force bool) (bool, error)
	// DiscoverEntityTemplates returns all templates (including variants) for an entity type.
	// Templates are named <type>.md (default) and <type>--<variant>.md (variants).
	DiscoverEntityTemplates(entityType string) ([]*markdown.EntityTemplate, error)

	// --- Path Helpers ---

	EntityFilePath(entityType, id string, meta *metamodel.Metamodel) string
	EntityTypeDir(entityType string, meta *metamodel.Metamodel) string

	// --- Change Notification ---

	// Watch starts watching for changes to stored data. The onChange callback
	// is called with batched change events after a debounce period. Returns a
	// stop function to shut down the watcher. Implementations may use
	// filesystem notifications, database triggers, polling, etc.
	Watch(opts WatchOptions, onChange func(events []model.ChangeEvent)) (stop func(), err error)

	// WatchWithHandle is like Watch but returns a WatchHandle that allows
	// pausing and resuming the watcher in addition to stopping it.
	WatchWithHandle(opts WatchOptions, onChange func(events []model.ChangeEvent)) (*WatchHandle, error)

	// --- Filesystem Access ---

	// FS returns the underlying filesystem for direct access when needed
	// (e.g., for attachment storage that operates on raw files).
	FS() storage.FS

	// --- Transactions ---

	// Transaction executes a function within a transaction context.
	// All write/delete operations are batched and applied atomically.
	// On error, all staged changes are rolled back.
	Transaction(fn func(tx Tx) error) error

	// FindOrphanedTempFiles scans for leftover .new files from interrupted transactions.
	FindOrphanedTempFiles() ([]string, error)

	// CleanupOrphanedTempFiles removes all orphaned .new temp files.
	// Returns the number of files cleaned up.
	CleanupOrphanedTempFiles() (int, error)
}

// WatchOptions configures optional parameters for Store.Watch.
type WatchOptions struct {
	// ExtraFiles lists additional files to watch beyond the standard set
	// (entities, relations, metamodel, views). This allows consumers to
	// watch app-specific config files (e.g. data-entry.yaml).
	ExtraFiles []string
	// ExtraDirs lists additional directories to watch beyond the standard set
	// (entities, relations). Useful for watching metamodel include directories.
	ExtraDirs []string
}

// Compile-time check: *Repository implements Store.
var _ Store = (*Repository)(nil)

// Repository provides domain-level CRUD for entities, relations, cache,
// metamodel, and templates. It does NOT hold the Graph — it only handles
// file persistence and path computation.
type Repository struct {
	fs    storage.FS
	paths *project.Context
	fio   *markdown.FileIO
}

// New creates a Repository with the given filesystem and project context.
func New(fs storage.FS, paths *project.Context) *Repository {
	return &Repository{
		fs:    fs,
		paths: paths,
		fio:   markdown.NewFileIO(fs),
	}
}

// Paths returns the project context.
func (r *Repository) Paths() *project.Context {
	return r.paths
}

// FS returns the underlying filesystem.
func (r *Repository) FS() storage.FS {
	return r.fs
}

// --- Entity CRUD ---

// ReadEntity reads and parses an entity file for the given type and ID.
func (r *Repository) ReadEntity(entityType, id string, meta *metamodel.Metamodel) (*model.Entity, error) {
	filePath := r.EntityFilePath(entityType, id, meta)
	if filePath == "" {
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
	return r.fio.ReadEntity(filePath, meta)
}

// WriteEntity writes an entity to its canonical file path.
// The path is computed from the entity's Type and ID using the metamodel's
// plural directory convention. Sets entity.FilePath before writing.
func (r *Repository) WriteEntity(entity *model.Entity, meta *metamodel.Metamodel) error {
	filePath := r.EntityFilePath(entity.Type, entity.ID, meta)
	if filePath == "" {
		return fmt.Errorf("unknown entity type: %s", entity.Type)
	}
	entity.FilePath = filePath
	return r.fio.WriteEntity(entity, filePath)
}

// DeleteEntity removes the entity file for the given type and ID.
func (r *Repository) DeleteEntity(entityType, id string, meta *metamodel.Metamodel) error {
	filePath := r.EntityFilePath(entityType, id, meta)
	if filePath == "" {
		return fmt.Errorf("unknown entity type: %s", entityType)
	}
	return r.fio.DeleteEntity(filePath)
}

// ListEntities loads all entities from the entities directory.
func (r *Repository) ListEntities(meta *metamodel.Metamodel) ([]*model.Entity, error) {
	return r.fio.LoadAllEntities(r.paths.EntitiesDir, meta)
}

// --- Relation CRUD ---

// ReadRelation reads and parses a relation file.
func (r *Repository) ReadRelation(from, relType, to string) (*model.Relation, error) {
	filePath := r.paths.RelationFilePath(from, relType, to)
	return r.fio.ReadRelation(filePath)
}

// WriteRelation writes a relation to its canonical file path.
// Sets relation.FilePath before writing.
func (r *Repository) WriteRelation(relation *model.Relation) error {
	filePath := r.paths.RelationFilePath(relation.From, relation.Type, relation.To)
	relation.FilePath = filePath
	return r.fio.WriteRelation(relation, filePath)
}

// DeleteRelation removes the relation file.
func (r *Repository) DeleteRelation(from, relType, to string) error {
	filePath := r.paths.RelationFilePath(from, relType, to)
	return r.fio.DeleteRelation(filePath)
}

// ListRelations loads all relations from the relations directory.
func (r *Repository) ListRelations() ([]*model.Relation, error) {
	return r.fio.LoadAllRelations(r.paths.RelationsDir)
}

// --- Sync ---

// Sync rebuilds the graph from all entity and relation files on disk.
func (r *Repository) Sync(meta *metamodel.Metamodel, g *graph.Graph) (*model.SyncResult, error) {
	data, err := r.fio.LoadSyncData(r.paths, meta)
	if err != nil {
		return nil, err
	}

	result := &model.SyncResult{
		Conflicted: data.Conflicted,
	}

	g.Clear()

	for _, entity := range data.Entities {
		g.AddNode(entity)
		result.EntitiesLoaded++
	}

	for _, relation := range data.Relations {
		if _, ok := g.GetNode(relation.From); !ok {
			result.Errors = append(result.Errors, &model.SyncError{
				File:    relation.FilePath,
				Message: "source entity not found: " + relation.From,
			})
			continue
		}
		if _, ok := g.GetNode(relation.To); !ok {
			result.Errors = append(result.Errors, &model.SyncError{
				File:    relation.FilePath,
				Message: "target entity not found: " + relation.To,
			})
			continue
		}
		g.AddEdge(relation)
		result.RelationsLoaded++
	}

	return result, nil
}

// --- Cache ---

// SaveCache writes the graph cache to the project's cache file.
func (r *Repository) SaveCache(g *graph.Graph) error {
	return g.SaveCache(r.paths.CachePath, r.fs)
}

// LoadCache reads the graph cache from the project's cache file.
func (r *Repository) LoadCache(g *graph.Graph) error {
	return g.LoadCache(r.paths.CachePath, r.fs)
}

// CacheExists returns true if the graph cache file exists.
func (r *Repository) CacheExists() bool {
	return graph.CacheExists(r.paths.CachePath, r.fs)
}

// --- Metamodel ---

// LoadMetamodel loads and parses the metamodel from the project's metamodel file.
// Returns a migration.Error if the file contains deprecated syntax that needs migration.
func (r *Repository) LoadMetamodel() (*metamodel.Metamodel, error) {
	detections, err := migration.Detect(r.paths.MetamodelPath, migration.FileTypeMetamodel, r.fs)
	if err != nil {
		return nil, err
	}
	if len(detections) > 0 {
		return nil, &migration.Error{
			FilePath:   r.paths.MetamodelPath,
			Detections: detections,
		}
	}
	return metamodel.Load(r.paths.MetamodelPath, r.fs)
}

// --- Views ---

// LoadViews loads and parses the views file from the project root.
// Returns an empty views file if views.yaml doesn't exist (views are optional).
func (r *Repository) LoadViews() (*views.File, error) {
	viewsPath := filepath.Join(r.paths.Root, "views.yaml")
	return views.Load(viewsPath, r.fs)
}

// --- Project Files ---

// ReadProjectFile reads a file relative to the project root.
func (r *Repository) ReadProjectFile(filename string) ([]byte, error) {
	return r.fs.ReadFile(filepath.Join(r.paths.Root, filename))
}

// WriteProjectFile writes a file relative to the project root.
func (r *Repository) WriteProjectFile(filename string, data []byte) error {
	return r.fs.WriteFile(filepath.Join(r.paths.Root, filename), data, 0o644)
}

// ReadCacheFile reads a file relative to the cache directory (.rela/).
func (r *Repository) ReadCacheFile(filename string) ([]byte, error) {
	return r.fs.ReadFile(filepath.Join(r.paths.CacheDir, filename))
}

// WriteCacheFile writes a file relative to the cache directory (.rela/).
// Creates the cache directory if it doesn't exist.
func (r *Repository) WriteCacheFile(filename string, data []byte) error {
	if err := r.fs.MkdirAll(r.paths.CacheDir, 0o755); err != nil {
		return err
	}
	return r.fs.WriteFile(filepath.Join(r.paths.CacheDir, filename), data, 0o644)
}

// --- Templates ---

// LoadEntityTemplate loads the template for the given entity type, or
// returns nil if no template exists.
func (r *Repository) LoadEntityTemplate(entityType string) (*markdown.Document, error) {
	return r.fio.LoadEntityTemplate(r.paths, entityType)
}

// LoadRelationTemplate loads the template for the given relation type, or
// returns nil if no template exists.
func (r *Repository) LoadRelationTemplate(relationType string) (*markdown.Document, error) {
	return r.fio.LoadRelationTemplate(r.paths, relationType)
}

// GenerateEntityTemplate generates a template file for the given entity type.
// If variant is non-empty, creates a variant template (e.g., type--variant.md).
// Returns true if a new template was created.
func (r *Repository) GenerateEntityTemplate(
	meta *metamodel.Metamodel, entityType, variant string, force bool,
) (bool, error) {
	return r.fio.GenerateEntityTemplate(r.paths, meta, entityType, variant, force)
}

// GenerateRelationTemplate generates a template file for the given relation type.
// Returns true if a new template was created.
func (r *Repository) GenerateRelationTemplate(
	meta *metamodel.Metamodel, relationType string, force bool,
) (bool, error) {
	return r.fio.GenerateRelationTemplate(r.paths, meta, relationType, force)
}

// DiscoverEntityTemplates returns all templates (including variants) for an entity type.
func (r *Repository) DiscoverEntityTemplates(entityType string) ([]*markdown.EntityTemplate, error) {
	return r.fio.DiscoverEntityTemplates(r.paths, entityType)
}

// --- Change Notification ---

// WatchHandle provides control over an active file watcher.
type WatchHandle struct {
	watcher *storage.Watcher
}

// Stop stops the file watcher and releases resources.
func (h *WatchHandle) Stop() {
	h.watcher.Stop()
}

// Pause temporarily stops processing file change events.
// Events that occur while paused are discarded.
func (h *WatchHandle) Pause() {
	h.watcher.Pause()
}

// Resume re-enables event processing after a Pause.
func (h *WatchHandle) Resume() {
	h.watcher.Resume()
}

// Watch starts watching for changes to entities, relations, metamodel, and
// views files. The onChange callback is called with batched change events.
// Returns a stop function to shut down the watcher.
func (r *Repository) Watch(
	opts WatchOptions, onChange func(events []model.ChangeEvent),
) (stop func(), err error) {
	handle, err := r.WatchWithHandle(opts, onChange)
	if err != nil {
		return nil, err
	}
	return handle.Stop, nil
}

// WatchWithHandle is like Watch but returns a WatchHandle that allows
// pausing and resuming the watcher in addition to stopping it.
func (r *Repository) WatchWithHandle(
	opts WatchOptions, onChange func(events []model.ChangeEvent),
) (*WatchHandle, error) {
	viewsPath := filepath.Join(r.paths.Root, "views.yaml")
	files := []string{r.paths.MetamodelPath, viewsPath}
	files = append(files, opts.ExtraFiles...)

	dirs := []string{r.paths.EntitiesDir, r.paths.RelationsDir}
	dirs = append(dirs, opts.ExtraDirs...)

	w, err := storage.NewWatcher(storage.WatchConfig{
		Dirs:       dirs,
		Files:      files,
		Extensions: []string{".md", ".yaml", ".yml"},
		Debounce:   200 * time.Millisecond,
		SkipHidden: true,
		OnChange:   onChange,
	})
	if err != nil {
		return nil, err
	}

	go w.Start()
	return &WatchHandle{watcher: w}, nil
}

// --- Path Helpers ---

// EntityFilePath computes the canonical file path for an entity.
// Returns "" if the entity type is not found in the metamodel.
func (r *Repository) EntityFilePath(entityType, id string, meta *metamodel.Metamodel) string {
	entDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	plural := entDef.Plural
	if plural == "" {
		plural = entityType + "s"
	}
	return r.paths.EntityFilePathWithPlural(plural, id)
}

// EntityTypeDir computes the directory for an entity type.
// Returns "" if the entity type is not found in the metamodel.
func (r *Repository) EntityTypeDir(entityType string, meta *metamodel.Metamodel) string {
	entDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	plural := entDef.Plural
	if plural == "" {
		plural = entityType + "s"
	}
	return r.paths.EntityTypeDirWithPlural(plural)
}
