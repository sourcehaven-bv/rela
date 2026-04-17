// Package store provides the storage abstraction for rela workspaces.
//
// The Store interface is limited to CRUD and write events. Query capabilities
// (search, trace, analytics) are separate services with their own interfaces.
// They build their state by subscribing to store events. Simple backends use
// generic implementations; smart backends (e.g. Postgres) provide native
// implementations sharing the same connection. This keeps the store contract
// small — new backends only implement data access, not every query algorithm.
package store

import (
	"context"
	"errors"
	"io"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Sentinel errors returned by store operations.
var (
	ErrNotFound      = errors.New("store: not found")
	ErrConflict      = errors.New("store: already exists")
	ErrHasRelations  = errors.New("store: entity has relations")
)

// Store is the primary storage abstraction. All mutations are atomic:
// the index is always consistent with the persisted state.
//
// Reads are cheap. Writes serialize internally — callers do not need
// external locking.
//
// Read methods return cloned entities/relations — callers own the
// returned values and may mutate them freely.
type Store interface {
	EntityReader
	EntityWriter
	RelationReader
	RelationWriter
	AttachmentManager
	Watcher
	Lifecycle
}

// EntityReader provides read access to entities.
type EntityReader interface {
	// GetEntity returns a single entity by ID.
	// Returns ErrNotFound if the entity does not exist.
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)

	// ListEntities returns an iterator over entities matching the query.
	// If an error is yielded, the iterator terminates.
	ListEntities(ctx context.Context, q EntityQuery) iter.Seq2[*entity.Entity, error]

	// CountEntities returns the number of entities matching the query.
	CountEntities(ctx context.Context, q EntityQuery) (int, error)

	// HighestID returns the highest sequential number found for the
	// given prefix (e.g. "FEAT" → 42 if FEAT-042 is the highest).
	// Returns 0 if no entities with the prefix exist.
	HighestID(ctx context.Context, prefix string) (int, error)

	// PropertyValues returns distinct values for a property, sorted by
	// frequency (most common first), up to limit results.
	PropertyValues(ctx context.Context, property string, limit int) ([]string, error)
}

// EntityQuery filters entity listings.
type EntityQuery struct {
	Type string   // filter by entity type (empty = all)
	IDs  []string // filter to specific IDs (empty = all)
}

// EntityWriter provides write access to entities.
type EntityWriter interface {
	// CreateEntity persists a new entity.
	// Returns ErrConflict if an entity with the same ID already exists.
	CreateEntity(ctx context.Context, e *entity.Entity) error

	// UpdateEntity persists changes to an existing entity.
	// Returns ErrNotFound if the entity does not exist.
	UpdateEntity(ctx context.Context, e *entity.Entity) error

	// DeleteEntity removes an entity and optionally its relations.
	// Returns ErrNotFound if the entity does not exist.
	DeleteEntity(ctx context.Context, id string, cascade bool) (*DeleteResult, error)

	// RenameEntity changes an entity's ID. All relations referencing the
	// old ID are updated atomically.
	// Returns ErrNotFound if the entity does not exist.
	// Returns ErrConflict if newID already exists.
	RenameEntity(ctx context.Context, oldID, newID string) (*RenameResult, error)
}

// DeleteResult describes what was removed.
type DeleteResult struct {
	DeletedEntities  []*entity.Entity
	DeletedRelations []*entity.Relation
}

// RenameResult describes what was updated during an entity rename.
type RenameResult struct {
	RelationsUpdated int
}

// RelationReader provides read access to relations.
type RelationReader interface {
	// GetRelation returns a single relation by its three-part key.
	// Returns ErrNotFound if the relation does not exist.
	GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error)

	// ListRelations returns an iterator over relations matching the query.
	// If an error is yielded, the iterator terminates.
	ListRelations(ctx context.Context, q RelationQuery) iter.Seq2[*entity.Relation, error]

	// CountRelations returns the number of relations matching the query.
	CountRelations(ctx context.Context, q RelationQuery) (int, error)
}

// RelationQuery filters relation listings.
type RelationQuery struct {
	From      string    // filter by source entity ID
	To        string    // filter by target entity ID
	Type      string    // filter by relation type
	EntityID  string    // filter by either endpoint (From OR To)
	Direction Direction // outgoing, incoming, or both
}

// Direction constrains relation queries to a specific direction.
type Direction int

const (
	DirectionBoth     Direction = iota // match both outgoing and incoming
	DirectionOutgoing                  // match only outgoing relations
	DirectionIncoming                  // match only incoming relations
)

// RelationWriter provides write access to relations.
type RelationWriter interface {
	// CreateRelation persists a new relation.
	// Returns ErrConflict if the relation already exists.
	CreateRelation(ctx context.Context, from, relType, to string, data *RelationData) (*entity.Relation, error)

	// UpdateRelation updates an existing relation's data.
	// Returns ErrNotFound if the relation does not exist.
	UpdateRelation(ctx context.Context, from, relType, to string, data RelationData) (*entity.Relation, error)

	// DeleteRelation removes a relation.
	// Returns ErrNotFound if the relation does not exist.
	DeleteRelation(ctx context.Context, from, relType, to string) error
}

// RelationData holds optional properties and content for a relation.
type RelationData struct {
	Properties map[string]interface{}
	Content    string
}

// AttachmentInfo describes a file attached to an entity.
type AttachmentInfo struct {
	EntityID    string
	Property    string
	FileName    string
	ContentType string
	Size        int64
}

// AttachmentManager provides file attachment operations.
type AttachmentManager interface {
	AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error
	ReadAttachment(ctx context.Context, entityID, property string) (io.ReadCloser, error)
	DeleteAttachment(ctx context.Context, entityID, property string) error
	ListAttachments(ctx context.Context, entityID string) ([]AttachmentInfo, error)
}

// SearchHit is a minimal result from a search operation.
type SearchHit struct {
	ID    string
	Type  string
	Title string
}

// Formatter checks whether an entity/relation's persisted representation
// is up to date with its canonical format. Optionally applies the format.
//
// This is NOT part of the Store interface — formatting is a persistence-layer
// concern specific to each backend. Stores that have a canonical serialized
// format (markdown files, YAML, etc.) provide their own Formatter.
type Formatter interface {
	// FormatEntity checks whether the entity's persisted form differs from its
	// canonical formatted form. If dryRun is false and it differs, the entity
	// is rewritten. Returns changed=true if a rewrite was (or would be) needed.
	FormatEntity(ctx context.Context, id string, dryRun bool) (changed bool, err error)

	// FormatRelation behaves like FormatEntity but for relations.
	FormatRelation(ctx context.Context, from, relType, to string, dryRun bool) (changed bool, err error)
}

// Searcher provides search and filtering over entities.
// This is NOT part of the Store interface — it is a separate query service
// that builds its state by subscribing to store events or by wrapping a
// SearchIndex. Smart backends (e.g. Postgres) can provide native
// implementations; simple backends use the generic implementation from
// the storesearch package.
type Searcher interface {
	Search(ctx context.Context, q SearchQuery) iter.Seq2[SearchHit, error]
}

// EntityObserver receives notifications when entities are created, updated,
// or deleted. Stores call observers synchronously after each write.
// Implementations must be safe for concurrent use.
//
// This is the hook mechanism for building derived state (search indexes,
// caches, projections) from store writes. Multiple observers can be
// registered on a single store.
type EntityObserver interface {
	// EntityPut is called when an entity is created or updated.
	EntityPut(e *entity.Entity) error

	// EntityDelete is called when an entity is removed.
	EntityDelete(id string) error
}

// SearchIndex is a pluggable full-text search index. It implements
// EntityObserver (the store calls EntityPut/EntityDelete on writes) and
// provides a Search method for querying. Implementations must be safe
// for concurrent use.
type SearchIndex interface {
	EntityObserver

	// Search returns entity IDs matching the query text, ordered by relevance.
	// limit ≤ 0 means no limit.
	Search(text string, limit int) ([]string, error)

	// Persistent returns true if the index survives process restarts.
	// When true, the index is only rebuilt when entity files have changed.
	// When false, all entities are re-indexed on every startup.
	Persistent() bool

	// Close releases any resources held by the index.
	Close() error
}

// SearchQuery describes a search request.
type SearchQuery struct {
	Text    string           // free-text search (ranked by relevance when set)
	Types   []string         // filter by entity types
	Filters []PropertyFilter // property-level filters
	Sort    []SortClause     // ordering (ignored when Text is set)
	Limit   int              // max results (0 = no limit)
}

// PropertyFilter matches entities by property value.
type PropertyFilter struct {
	Property string
	Value    string
	Op       FilterOp
}

// FilterOp defines how a property filter matches.
type FilterOp int

const (
	FilterEq       FilterOp = iota // exact match (default)
	FilterNe                       // not equal
	FilterContains                 // substring match
	FilterGt                       // greater than
	FilterLt                       // less than
	FilterGte                      // greater than or equal
	FilterLte                      // less than or equal
	FilterIn                       // value is one of a comma-separated set
	FilterExists                   // property is set (Value ignored)
	FilterNotExists                // property is not set (Value ignored)
)

// SortClause defines a single sort dimension.
type SortClause struct {
	Field     string
	Direction SortDirection
}

// SortDirection is ascending or descending.
type SortDirection int

const (
	SortAsc  SortDirection = iota
	SortDesc
)

// Event represents a change that occurred in the store.
type Event struct {
	Op           EventOp
	EntityType   string
	EntityID     string
	RelationType string
	From         string
	To           string
}

// EventOp identifies the kind of change.
type EventOp int

const (
	EventEntityCreated EventOp = iota
	EventEntityUpdated
	EventEntityDeleted
	EventRelationCreated
	EventRelationUpdated
	EventRelationDeleted
)

// Watcher provides change notification.
//
// Events are sent asynchronously — never under a store lock. If the
// subscriber's channel buffer is full, events are dropped.
type Watcher interface {
	Subscribe(bufSize int) (events <-chan Event, cancel func())
}

// Lifecycle manages store shutdown.
type Lifecycle interface {
	Close() error
}

// TypeResolver maps entity IDs and aliases to canonical type names.
// Required by backends that infer type from ID prefixes or file paths.
type TypeResolver interface {
	InferEntityType(id string) string
	ResolveAlias(name string) string
}

// EntityTypeSchema holds the storage-relevant configuration for an entity type.
type EntityTypeSchema struct {
	Plural        string
	PropertyOrder []string
}
