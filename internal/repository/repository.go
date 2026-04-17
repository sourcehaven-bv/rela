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

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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

	// --- Path Helpers ---

	EntityFilePath(entityType, id string, meta *metamodel.Metamodel) string
	EntityTypeDir(entityType string, meta *metamodel.Metamodel) string

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
