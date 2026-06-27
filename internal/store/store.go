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
	"fmt"
	"io"
	"iter"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Sentinel errors returned by store operations.
var (
	ErrNotFound     = errors.New("store: not found")
	ErrConflict     = errors.New("store: already exists")
	ErrHasRelations = errors.New("store: entity has relations")
	// ErrAttachmentTooLarge is returned by AttachFile when the supplied
	// bytes exceed MaxAttachmentBytes. Every backend enforces this as a
	// backstop so no storage path is ever unbounded; the HTTP/API layer
	// caps at its own ingress for a clean 413 before reaching the store.
	ErrAttachmentTooLarge = errors.New("store: attachment too large")
)

// MaxAttachmentBytes is the backstop cap every store backend enforces on
// a single attachment's bytes. It is a defense-in-depth guard, not the
// product policy limit — the API layer caps uploads at its own (usually
// equal or lower) ingress. 64 MiB comfortably covers the expected use
// (images, PDFs, office documents); PostgreSQL also caps BYTEA near 1 GB.
const MaxAttachmentBytes = 64 << 20

// CapAttachmentReader wraps r so reads fail with ErrAttachmentTooLarge
// once they exceed `limit` bytes. It is the single shared bounded-reader
// behind both the store backstop (every backend, at MaxAttachmentBytes)
// and the API layer's per-request cap (at the configured upload limit),
// so the off-by-one lives in one place. Unlike io.LimitReader (which
// reports io.EOF at the boundary, indistinguishable from a genuine short
// file), this surfaces an explicit error so callers can map it to a 413
// and clean up any partial write. The too-large error deliberately wins
// over any underlying read error at the boundary.
func CapAttachmentReader(r io.Reader, limit int64) io.Reader {
	return &cappedAttachmentReader{r: r, remaining: limit}
}

type cappedAttachmentReader struct {
	r         io.Reader
	remaining int64
}

func (l *cappedAttachmentReader) Read(p []byte) (int, error) {
	if l.remaining < 0 {
		return 0, ErrAttachmentTooLarge
	}
	// Allow reading one extra byte past the cap so a file exactly at the
	// limit succeeds but anything larger trips on the next read.
	if int64(len(p)) > l.remaining+1 {
		p = p[:l.remaining+1]
	}
	n, err := l.r.Read(p)
	l.remaining -= int64(n)
	if l.remaining < 0 {
		return n, ErrAttachmentTooLarge
	}
	return n, err
}

// ValidateFileName rejects attachment file names that would corrupt the
// per-file storage key / path. The file name is a key segment (and an
// on-disk path leaf in fsstore), so it must not be empty, contain a path
// separator or NUL, or be a directory-traversal token. Callers should
// normalize with [NormalizeFileName] before storing; this is the hard gate
// every backend's AttachFile applies.
func ValidateFileName(name string) error {
	if name == "" {
		return errors.New("store: empty attachment file name")
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("store: attachment file name %q contains a path separator", name)
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("store: attachment file name %q contains a NUL byte", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("store: attachment file name %q is a directory reference", name)
	}
	return nil
}

// NormalizeFileName reduces an arbitrary upload name to a safe storage
// key: it takes the base name (stripping any path), replaces path
// separators and control characters, and trims surrounding dots/spaces. It
// preserves the extension and the human-readable stem so the stored name
// still resembles what the user uploaded. Returns "file" if nothing usable
// remains.
func NormalizeFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	const firstPrintable = 0x20 // chars below this are ASCII control codes
	var b strings.Builder
	for _, r := range name {
		if r == '/' || r == '\\' || r == 0 || r < firstPrintable {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.Trim(b.String(), " .")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "file"
	}
	return cleaned
}

// SuffixOnCollision returns name unchanged if exists(name) is false;
// otherwise it appends a " (n)" counter before the extension until it
// finds a free name (report.pdf -> "report (1).pdf" -> "report (2).pdf"),
// mirroring how a file manager handles duplicate drops so a multi-file
// upload never silently overwrites a same-named file.
func SuffixOnCollision(name string, exists func(string) bool) string {
	if !exists(name) {
		return name
	}
	ext := attachmentExt(name)
	stem := name[:len(name)-len(ext)]
	// Terminates in practice: callers only suffix when the property is under
	// its (small, validated) `max` cap, so at most `max` names are taken.
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, n, ext)
		if !exists(candidate) {
			return candidate
		}
	}
}

// attachmentExt returns the trailing extension (including the dot), or ""
// — like filepath.Ext but treating a leading-dot name (".bashrc") as
// having no extension.
func attachmentExt(name string) string {
	for i := len(name) - 1; i >= 0 && name[i] != '/'; i-- {
		if name[i] == '.' {
			if i == 0 {
				return ""
			}
			return name[i:]
		}
	}
	return ""
}

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
	GraphQueryer
	AttachmentManager
	Watcher
	Lifecycle
	Freshness
}

// Freshness exposes the store's overall "last modified" timestamp, covering
// entity and relation writes. Consumers maintaining derived state (search
// indexes, graph caches, projections) compare this against their own
// "last synced" timestamp to decide whether to rebuild.
type Freshness interface {
	// LastModified returns the latest mutation time across all entities and
	// relations in the store. Returns a zero time if the store is empty.
	LastModified(ctx context.Context) (time.Time, error)
}

// EntityReader provides read access to entities.
//
// List operations return results in stable, implementation-defined order:
// implementations MUST return the same order across calls when the
// underlying data has not changed, so cursors remain valid between pages.
// The default order is ascending by ID.
type EntityReader interface {
	// GetEntity returns a single entity by ID.
	// Returns ErrNotFound if the entity does not exist.
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)

	// ListEntities returns an iterator over entities matching the query.
	// If an error is yielded, the iterator terminates. Cursor and Limit
	// on the query are ignored — use ListEntitiesPage for pagination.
	ListEntities(ctx context.Context, q EntityQuery) iter.Seq2[*entity.Entity, error]

	// ListEntitiesPage returns a page of entities matching the query.
	// When q.Limit == 0, the full result set is returned in one page
	// (NextCursor is always empty). When q.Limit > 0, at most Limit
	// entities are returned; NextCursor is non-empty iff more results
	// exist. Callers resume by setting q.Cursor to the returned
	// NextCursor on the next call, keeping other query fields identical.
	//
	// Cursors are opaque — callers MUST NOT parse or construct them.
	// A cursor is only valid for the same query on the same store;
	// behavior with a mismatched cursor is implementation-defined.
	ListEntitiesPage(ctx context.Context, q EntityQuery) (Page[*entity.Entity], error)

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
	Type   string   // filter by entity type (empty = all)
	IDs    []string // filter to specific IDs (empty = all)
	Cursor string   // pagination cursor from a previous page (empty = start); ignored by ListEntities
	Limit  int      // max entities per page (0 = no limit); ignored by ListEntities
}

// Page holds a single page of results from a paginated list call.
// NextCursor is empty when no further pages exist.
type Page[T any] struct {
	Items      []T
	NextCursor string
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
//
// List operations return results in stable, implementation-defined order;
// see EntityReader for the full contract.
type RelationReader interface {
	// GetRelation returns a single relation by its three-part key.
	// Returns ErrNotFound if the relation does not exist.
	GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error)

	// ListRelations returns an iterator over relations matching the query.
	// If an error is yielded, the iterator terminates. Cursor and Limit
	// on the query are ignored — use ListRelationsPage for pagination.
	ListRelations(ctx context.Context, q RelationQuery) iter.Seq2[*entity.Relation, error]

	// ListRelationsPage returns a page of relations matching the query.
	// See ListEntitiesPage for the cursor/limit contract.
	ListRelationsPage(ctx context.Context, q RelationQuery) (Page[*entity.Relation], error)

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
	Cursor    string    // pagination cursor from a previous page (empty = start); ignored by ListRelations
	Limit     int       // max relations per page (0 = no limit); ignored by ListRelations
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

// AttachmentManager provides file attachment operations. A property can
// hold multiple attachments, each keyed by its (normalized) file name —
// so reads and deletes target a specific (entityID, property, fileName).
// AttachFile appends; it does not overwrite other files on the property.
// Enforcing a per-property cap (the metamodel `max`) and replace-at-1
// semantics is the write path's job, not the store's.
type AttachmentManager interface {
	AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error
	ReadAttachment(ctx context.Context, entityID, property, fileName string) (io.ReadCloser, error)
	DeleteAttachment(ctx context.Context, entityID, property, fileName string) error
	ListAttachments(ctx context.Context, entityID string) ([]AttachmentInfo, error)
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

// EntityObserver receives notifications when entities are created, updated,
// deleted, or renamed. Stores call observers synchronously after each write.
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

	// EntityRenamed is called when an entity's ID changes. The renamed
	// argument carries the entity AFTER the rename (renamed.ID == newID)
	// so content-driven observers (search indexes, projections that
	// hold a copy) have everything they need without a follow-up
	// store lookup, and ID-keyed observers (waiver stores, anything
	// that stores references by entity ID) can rewrite those
	// references in one step.
	//
	// Rename emits EXACTLY this one callback — not EntityDelete(oldID)
	// + EntityPut(renamed). Implementations of search-index-style
	// backends should atomically delete the old key and index the new
	// content in their EntityRenamed body.
	EntityRenamed(oldID string, renamed *entity.Entity) error
}

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
