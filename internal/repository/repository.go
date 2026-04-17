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
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Store defines the domain-level persistence contract for entities,
// relations, cache, metamodel, and templates. Implementations may
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

	Sync(meta *metamodel.Metamodel) (*graph.Graph, *model.SyncResult, error)

	// --- Metamodel ---

	// LoadMetamodel loads and parses the metamodel from the project's metamodel file.
	// The returned []string contains the absolute paths of all files that were read
	// (metamodel.yaml plus any include files).
	LoadMetamodel() (*metamodel.Metamodel, []string, error)

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

	// LoadEntityTemplate loads the default template for the given entity type.
	// Returns nil if no template exists (templates are optional).
	LoadEntityTemplate(entityType string) (*markdown.Document, error)
	// LoadEntityTemplateVariant loads a template variant for the given entity type.
	// If variant is empty, loads the default template (<type>.md).
	// Otherwise loads <type>--<variant>.md.
	LoadEntityTemplateVariant(entityType, variant string) (*markdown.Document, error)
	LoadRelationTemplate(relationType string) (*markdown.Document, error)
	GenerateEntityTemplate(meta *metamodel.Metamodel, entityType, variant string, force bool) (bool, error)
	GenerateRelationTemplate(meta *metamodel.Metamodel, relationType string, force bool) (bool, error)
	// DiscoverEntityTemplates returns all templates (including variants) for an entity type.
	// Templates are named <type>.md (default) and <type>--<variant>.md (variants).
	DiscoverEntityTemplates(entityType string) ([]*model.EntityTemplate, error)

	// --- Path Helpers ---

	EntityFilePath(entityType, id string, meta *metamodel.Metamodel) string
	EntityTypeDir(entityType string, meta *metamodel.Metamodel) string

	// --- Change Notification ---

	// Watch starts watching for changes to stored data. The onChange callback
	// is called with batched change events after a debounce period. Returns a
	// stop function to shut down the watcher. Implementations may use
	// filesystem notifications, database triggers, polling, etc.
	Watch(opts WatchOptions, onChange func(events []ChangeEvent)) (stop func(), err error)

	// WatchWithHandle is like Watch but returns a WatchHandle that allows
	// pausing and resuming the watcher in addition to stopping it.
	WatchWithHandle(opts WatchOptions, onChange func(events []ChangeEvent)) (*WatchHandle, error)

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
// Properties are written in the order defined in the metamodel.
func (r *Repository) WriteEntity(entity *model.Entity, meta *metamodel.Metamodel) error {
	filePath := r.EntityFilePath(entity.Type, entity.ID, meta)
	if filePath == "" {
		return fmt.Errorf("unknown entity type: %s", entity.Type)
	}
	entity.FilePath = filePath

	// Get property order from metamodel if available
	var propertyOrder []string
	if entityDef, ok := meta.GetEntityDef(entity.Type); ok {
		propertyOrder = entityDef.GetPropertyOrder()
	}

	return r.fio.WriteEntity(entity, filePath, propertyOrder)
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

// Sync loads all entities and relations from disk and returns them as a
// freshly-built graph. The caller is responsible for publishing the new
// graph (e.g. by atomically swapping a pointer). Sync never mutates any
// pre-existing graph — if it fails, the caller's previous graph remains
// valid.
//
// This contract is what allows Workspace.Reload to satisfy readers holding
// a pre-reload graph snapshot: the old graph is never touched, so those
// readers continue to see a coherent (if stale) world until they reload
// the snapshot.
func (r *Repository) Sync(meta *metamodel.Metamodel) (*graph.Graph, *model.SyncResult, error) {
	data, err := r.fio.LoadSyncData(r.paths.EntitiesDir, r.paths.RelationsDir, meta)
	if err != nil {
		return nil, nil, err
	}

	result := &model.SyncResult{
		Conflicted: data.Conflicted,
	}

	g := graph.New()

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

	return g, result, nil
}

// --- Metamodel ---

// LoadMetamodel loads and parses the metamodel from the project's metamodel file.
// Returns a migration.Error if the file contains deprecated syntax that needs migration.
// The returned []string contains the absolute paths of all files that were read
// (metamodel.yaml plus any include files).
func (r *Repository) LoadMetamodel() (*metamodel.Metamodel, []string, error) {
	detections, err := migration.Detect(r.paths.MetamodelPath, migration.FileTypeMetamodel, r.fs)
	if err != nil {
		return nil, nil, err
	}
	if len(detections) > 0 {
		return nil, nil, &migration.Error{
			FilePath:   r.paths.MetamodelPath,
			Detections: detections,
		}
	}
	return metamodel.Load(r.paths.MetamodelPath, r.fs)
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
//
// The filename must be a single path component — `..`, `/`, `\`, and NUL
// bytes are rejected. Today all callers pass hardcoded names, so this check
// is purely defensive against future regressions.
func (r *Repository) WriteCacheFile(filename string, data []byte) error {
	if err := validateCacheFilename(filename); err != nil {
		return err
	}
	fullPath := filepath.Join(r.paths.CacheDir, filename)
	if err := r.fs.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return r.fs.WriteFile(fullPath, data, 0o644)
}

// validateCacheFilename ensures name is a relative path that stays inside the
// cache directory. A subdirectory is allowed (e.g. "documents/x.html") because
// real callers cache things in nested groups, but absolute paths, `..`
// segments, backslashes, control characters (including NUL), and Windows
// drive-letter syntax are rejected.
func validateCacheFilename(name string) error {
	if name == "" {
		return fmt.Errorf("cache filename: must not be empty")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("cache filename: control character (including NUL) not allowed")
		}
	}
	if strings.ContainsRune(name, '\\') {
		return fmt.Errorf("cache filename: backslash not allowed (use forward slash)")
	}
	if strings.HasPrefix(name, "/") {
		return fmt.Errorf("cache filename: must be relative")
	}
	// Reject any `..` or `.` segment regardless of position. Cleaning would
	// silently collapse them, hiding malicious intent.
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("cache filename: traversal or empty segment not allowed")
		}
	}
	// Reject Windows drive-letter syntax (e.g. "c:foo.yaml") which would
	// be misinterpreted on Windows even after passing the other checks.
	if len(name) >= 2 && name[1] == ':' {
		return fmt.Errorf("cache filename: drive letter not allowed")
	}
	return nil
}

// --- Templates ---

// LoadEntityTemplate loads the template for the given entity type, or
// returns nil if no template exists.
func (r *Repository) LoadEntityTemplate(entityType string) (*markdown.Document, error) {
	return r.fio.LoadEntityTemplate(r.paths.EntityTemplatePath(entityType))
}

// LoadEntityTemplateVariant loads a template variant for the given entity type.
// If variant is empty, loads the default template (<type>.md).
// Otherwise loads <type>--<variant>.md.
func (r *Repository) LoadEntityTemplateVariant(entityType, variant string) (*markdown.Document, error) {
	return r.fio.LoadEntityTemplate(r.paths.EntityTemplateVariantPath(entityType, variant))
}

// LoadRelationTemplate loads the template for the given relation type, or
// returns nil if no template exists.
func (r *Repository) LoadRelationTemplate(relationType string) (*markdown.Document, error) {
	return r.fio.LoadRelationTemplate(r.paths.RelationTemplatePath(relationType))
}

// GenerateEntityTemplate generates a template file for the given entity type.
// If variant is non-empty, creates a variant template (e.g., type--variant.md).
// Returns true if a new template was created.
func (r *Repository) GenerateEntityTemplate(
	meta *metamodel.Metamodel, entityType, variant string, force bool,
) (bool, error) {
	path := r.paths.EntityTemplateVariantPath(entityType, variant)
	return r.fio.GenerateEntityTemplate(path, meta, entityType, force)
}

// GenerateRelationTemplate generates a template file for the given relation type.
// Returns true if a new template was created.
func (r *Repository) GenerateRelationTemplate(
	meta *metamodel.Metamodel, relationType string, force bool,
) (bool, error) {
	path := r.paths.RelationTemplatePath(relationType)
	return r.fio.GenerateRelationTemplate(path, meta, relationType, force)
}

// DiscoverEntityTemplates returns all templates (including variants) for an entity type.
func (r *Repository) DiscoverEntityTemplates(entityType string) ([]*model.EntityTemplate, error) {
	return r.fio.DiscoverEntityTemplates(r.paths.EntityTemplatesDir, entityType)
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

// AddFile adds an individual file to the watch list at runtime.
func (h *WatchHandle) AddFile(path string) error {
	return h.watcher.AddFile(path)
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

// Watch starts watching for changes to entities, relations, and metamodel
// files. The onChange callback is called with batched change events.
// Returns a stop function to shut down the watcher.
func (r *Repository) Watch(
	opts WatchOptions, onChange func(events []ChangeEvent),
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
	opts WatchOptions, onChange func(events []ChangeEvent),
) (*WatchHandle, error) {
	files := []string{r.paths.MetamodelPath}
	files = append(files, opts.ExtraFiles...)

	dirs := []string{r.paths.EntitiesDir, r.paths.RelationsDir}
	dirs = append(dirs, opts.ExtraDirs...)

	w, err := storage.NewWatcher(storage.WatchConfig{
		Dirs:       dirs,
		Files:      files,
		Extensions: []string{".md", ".yaml", ".yml"},
		Debounce:   200 * time.Millisecond,
		SkipHidden: true,
		OnChange: func(events []storage.ChangeEvent) {
			onChange(convertEvents(events))
		},
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
