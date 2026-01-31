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

// FS returns the underlying filesystem.
func (r *Repository) FS() storage.FS {
	return r.fs
}

// Paths returns the project context.
func (r *Repository) Paths() *project.Context {
	return r.paths
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
func (r *Repository) Sync(meta *metamodel.Metamodel, g *graph.Graph) (*markdown.SyncResult, error) {
	return r.fio.SyncFromFiles(r.paths, meta, g)
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
func (r *Repository) LoadMetamodel() (*metamodel.Metamodel, error) {
	return metamodel.Load(r.paths.MetamodelPath, r.fs)
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
// Returns true if a new template was created.
func (r *Repository) GenerateEntityTemplate(
	meta *metamodel.Metamodel, entityType string, force bool,
) (bool, error) {
	return r.fio.GenerateEntityTemplate(r.paths, meta, entityType, force)
}

// GenerateRelationTemplate generates a template file for the given relation type.
// Returns true if a new template was created.
func (r *Repository) GenerateRelationTemplate(
	meta *metamodel.Metamodel, relationType string, force bool,
) (bool, error) {
	return r.fio.GenerateRelationTemplate(r.paths, meta, relationType, force)
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
