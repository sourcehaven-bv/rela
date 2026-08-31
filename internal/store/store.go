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
// external locking. Multi-write operations that must not interleave
// with other writers group their writes with [Transactor.Tx].
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
	Transactor
}

// Transactor groups writes into one serialized unit (DEC-8UIL0).
//
// Tx runs fn with a transaction-bound view of the store. The contract
// specifies behavior, not mechanism — each backend meets it with its
// native machinery:
//
//   - Writes made through the view are serialized against every other
//     writer and every other Tx for the full duration of fn —
//     cross-process where the backend can see other processes
//     (pgstore: native transaction + advisory lock), in-process
//     otherwise (fsstore/memstore: write mutex; single-user
//     deployments by nature).
//   - Reads through the view observe fn's own writes.
//   - An error from fn is returned unchanged. Where the backend
//     supports rollback (pgstore), the transaction's writes are
//     discarded and no events are delivered; fsstore/memstore keep
//     the writes already made (no rollback — deliberate reduced
//     guarantee, they cannot promise crash atomicity either).
//   - A nested Tx on the view joins the open transaction; there is no
//     nested-transaction API.
//
// All store calls inside fn MUST go through the view, and the view
// must not escape fn or be shared across goroutines — writes on the
// outer store from inside fn deadlock (fs/mem) or bypass the
// transaction (pg). Beware that using an ESCAPED view after Tx
// returns is the one misuse that fails silently: on fs/mem it writes
// without the serialization lock (racing any open Tx), on pg it
// errors against a closed transaction. Do not perform slow external
// I/O inside fn: on pgstore the whole deployment's writers wait.
type Transactor interface {
	Tx(ctx context.Context, fn func(Store) error) error
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
	Properties map[string]any
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

// EntityHeader is an entity WITHOUT its body content.
//
// It exists so whole-store scans that only need identity and properties —
// analyze checks, ID generation, list projections — never materialize
// markdown bodies they will not read. On a 20k-entity store with ~100 KB
// bodies that is the difference between ~2 GB and a few MB.
//
// A SEPARATE TYPE, not an [entity.Entity] with Content left empty, and that
// is the whole point (TKT-1ESTYJ). A half-populated Entity satisfies every
// interface an Entity satisfies and lies to all of them: hand one to
// dataentry's computeEntityETag, which hashes e.Content, and it returns a
// well-formed ETag for the wrong bytes — silently breaking conditional
// requests and caching. There are hundreds of `.Content` reads across the
// tree; a bool flag on [EntityQuery] would make every one of them a latent
// bug. With a distinct type the compiler rejects the mistake instead.
//
// Do not add a Content field. If a caller needs the body it wants
// [EntityReader.GetEntity] or [EntityReader.ListEntities], and the fact that
// it must say so explicitly is the guardrail working.
type EntityHeader struct {
	ID         string
	Type       string
	Properties map[string]any
	UpdatedAt  time.Time

	// Redacted mirrors [entity.Entity.Redacted]: the properties withheld
	// from the reading principal by field-level ACL. Carried so a gated
	// header read reports redaction exactly like a gated entity read —
	// a header must never look MORE complete than the entity it projects.
	Redacted []string
}

// HeaderReader lists entities without their body content.
//
// OPTIONAL capability, type-asserted at the call site like [Formatter] —
// not part of [Store]. Backends that can project the body away in the query
// (pgstore: omit the content column) implement it; others are served by
// [ListEntityHeaders], the generic fallback that drops content in-process.
//
// Callers should use [ListEntityHeaders] rather than asserting themselves,
// so the fallback stays a detail of this package.
type HeaderReader interface {
	// ListEntityHeaders returns an iterator over content-free headers for
	// entities matching the query. Ordering and error semantics match
	// [EntityReader.ListEntities]; Cursor and Limit are likewise ignored.
	ListEntityHeaders(ctx context.Context, q EntityQuery) iter.Seq2[EntityHeader, error]
}

// HeaderOf projects an entity onto its content-free header.
//
// The Properties map is shared, not cloned: every caller of this and of
// [ListEntityHeaders] treats headers as read-only, and cloning a map per row
// would reintroduce a per-entity allocation on the very path that exists to
// avoid one. Do not mutate a header's Properties.
func HeaderOf(e *entity.Entity) EntityHeader {
	return EntityHeader{
		ID:         e.ID,
		Type:       e.Type,
		Properties: e.Properties,
		UpdatedAt:  e.UpdatedAt,
		Redacted:   e.Redacted,
	}
}

// ListEntityHeaders lists content-free entity headers from any reader.
//
// Uses the reader's native [HeaderReader] when it has one, so the body never
// leaves the backend; otherwise falls back to [EntityReader.ListEntities] and
// projects each row as it is yielded. The fallback bounds RETENTION (rows are
// converted and released one at a time, never accumulated) but not transfer —
// a backend without the capability still reads bodies off disk or the wire.
// Do not describe the fallback as bounding I/O.
func ListEntityHeaders(
	ctx context.Context, r EntityReader, q EntityQuery,
) iter.Seq2[EntityHeader, error] {
	if hr, ok := r.(HeaderReader); ok {
		return hr.ListEntityHeaders(ctx, q)
	}
	return func(yield func(EntityHeader, error) bool) {
		for e, err := range r.ListEntities(ctx, q) {
			if err != nil {
				yield(EntityHeader{}, err)
				return
			}
			if !yield(HeaderOf(e), nil) {
				return
			}
		}
	}
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

// VersionOp is the operation that produced an entity version, mirroring the
// write that triggered capture.
type VersionOp string

const (
	VersionOpCreate VersionOp = "create"
	VersionOpUpdate VersionOp = "update"
	VersionOpRename VersionOp = "rename"
	VersionOpDelete VersionOp = "delete"
	// VersionOpPurge is a no-content tombstone marker written when a lineage's
	// history is deliberately purged while its LIVE row still exists (a
	// --force-live purge). It carries NO snapshot content — only the op,
	// principal, and vseq — and exists so the reconciliation sweep recognizes
	// "this lineage was purged on purpose" and does NOT re-capture the live
	// content as a fresh version (TKT-BW6UUL RR-SH28E). It is never produced by
	// an ordinary write.
	VersionOpPurge VersionOp = "purge"
)

// VersionMeta is a single row of an entity's version timeline, without the
// snapshot body/properties — enough to render a history list. Version is the
// human-facing 1-based ordinal within the entity's lineage (computed at read
// time), newest last.
type VersionMeta struct {
	Version       int
	Op            VersionOp
	PrevID        string // set only for VersionOpRename: the entity's former ID
	Type          string
	ContentHash   string
	SchemaHash    string
	PrincipalUser string
	PrincipalTool string
	TriggeredBy   string
	CreatedAt     time.Time
}

// VersionSnapshot is a full captured version: its metadata plus the entity
// content and properties as they were, and the render-schema projection (as
// stored JSON) the snapshot was taken under. Rendering a snapshot resolves
// display/typing against Projection, not the live metamodel, so a historical
// version renders faithfully even after the schema drifts.
type VersionSnapshot struct {
	VersionMeta
	Content    string
	Properties map[string]any
	Projection []byte // the schema_versions.projection JSON for SchemaHash
}

// VersionInput is one entity version to persist via [VersionWriter]. It is the
// store-facing shape of a synchronous capture (rename/delete): the snapshot
// state, its op, the render-schema projection it was taken under (hash + JSON,
// deduped into the backend's schema store), and attribution. PrevID is set only
// for VersionOpRename.
type VersionInput struct {
	EntityID      string
	Op            VersionOp
	PrevID        string
	Type          string
	Content       string
	Properties    map[string]any
	SchemaHash    string
	Projection    []byte
	PrincipalUser string
	PrincipalTool string
	TriggeredBy   string
}

// VersionWriter persists a captured entity version. Like HistoryReader it is an
// optional, backend-specific capability (pgstore only). The entitymanager's
// synchronous version hook dispatches rename/delete captures here via a
// wiring-supplied adapter; the store never learns the Principal by any other
// route (it arrives inside VersionInput, populated from ctx at the boundary).
type VersionWriter interface {
	// WriteVersion persists one version row. It is best-effort from the
	// caller's perspective (the entitymanager logs and swallows the error),
	// but the implementation should still return a real error for diagnosis.
	WriteVersion(ctx context.Context, in VersionInput) error
}

// TypeWatermark reports the newest change sequence for an entity type, so a
// caller can answer "has anything of this type changed?" without reading the
// rows.
//
// Optional and backend-specific, like [HistoryReader] and [VersionWriter]:
// callers type-assert a Store to it and degrade when the assertion fails. Only
// backends with a monotonic per-write sequence can implement it — pgstore has
// `rela_seq`; fsstore has only wall-clock mtimes and no ordering, so it
// deliberately does not.
//
// # Why this exists
//
// The CalDAV collection tag (`getctag`) is polled by every client on every
// cycle, and computing it today renders the entire collection to hash the
// per-entry ETags — the exact work the tag exists to let clients SKIP. That is
// tolerable for a handful of configured collections and quadratic for
// graph-driven ones (one collection per project ⇒ P renders per poll).
//
// A watermark answers the same question with an index-only `max(seq)`.
//
// # Deletions are included, and that is load-bearing
//
// The value MUST account for hard-deleted rows. `max(seq)` over live rows alone
// can go DOWN when the newest row is deleted, and a tag that moves backwards
// makes a client that already saw the higher value stop polling — it is
// permanently stale with no way to notice. Implementations combine the live-row
// maximum with the deletion-tombstone maximum.
//
// # Type scope is deliberate, and over-triggers
//
// The watermark is scoped by entity TYPE only, never by a collection's filter or
// by the caller's ACL. A deletion tombstone records just (kind, id, type) — the
// deleted row's properties and relations are gone — so a narrower scope cannot
// be reconstructed once the row is removed.
//
// The consequence: any write to the type moves the watermark for every consumer
// of that type, and a client re-enumerates to discover nothing changed. That is
// the SAFE direction. A spurious re-sync costs one listing and self-corrects; a
// missed change strands a client forever. Do not "optimize" this into a
// per-collection or per-principal scope without solving the tombstone problem
// first.
//
// # The over-triggering is also a disclosure, and it is accepted
//
// The paragraph above argues that over-triggering is FUNCTIONALLY safe. That is
// a different question from whether it is CONFIDENTIAL, and the second question
// only appears once one type is exposed through several differently-authorized
// collections — which graph-driven CalDAV collections do (`project_tasks--PRJ-1`
// and `project_tasks--PRJ-2` are one type, two ACLs).
//
// Those two collections do not share a tag VALUE; the CalDAV layer hashes the
// collection name in alongside the sequence. They do share the tag's only
// varying input, so they move at the same TIME. A principal who may read PRJ-1
// alone sees their ctag advance whenever ANY entity of the type is written,
// including in a project the ACL hides from them.
//
// Size it before weighing it. The observer learns one bit — "something of this
// type changed" — with no id, no content, no count, and no timing resolution
// finer than their own poll interval. They already knew the deployment has that
// type; that is why they have a collection over it. It is the class of signal
// any shared multi-tenant system emits through caches and latency.
//
// Accepted as a documented residual risk (GitHub issue #1370, CONTROL-5-15,
// severity low). The alternatives are worse, and it is worth knowing WHY before
// proposing one again:
//
//   - A per-principal or per-driver watermark is not merely unimplemented, it is
//     unavailable. It could not see deletions inside its own scope — the
//     tombstone does not record the scope — so its max(seq) would run BACKWARDS
//     when the newest row in scope is deleted. That is the failure the section
//     above calls unrecoverable: a client that already saw the higher value
//     stops polling and is stale forever. Trading a one-bit signal for silent
//     data loss is not a security improvement.
//   - Teaching the tombstone to remember the driver relation would make that
//     scope reconstructible, and it is the option that looks cheapest from
//     pgstore's two-column INSERT. It is not a schema question. The row would
//     then assert "this deleted subject belonged to that project", so a
//     relational fact about a person SURVIVES their deletion — which inverts
//     what deletion is for and is a GDPR/AVG question before it is a
//     performance one. Answer that first, or do not start.
//   - Falling back to per-entry ETag hashing is not an alternative design; it is
//     what happens already when the store is not a TypeWatermark, and it costs
//     exactly what "Why this exists" above says this interface exists to avoid.
type TypeWatermark interface {
	// EntityTypeWatermark returns a monotonic value that changes whenever any
	// entity of entityType is created, updated, renamed or deleted.
	//
	// Returns 0 when the type has never had a row — a legitimate value, not an
	// error, and stable for as long as that stays true.
	EntityTypeWatermark(ctx context.Context, entityType string) (int64, error)
}

// HistoryReader reads an entity's captured version history. Like Formatter it
// is NOT part of the Store interface — content versioning is a backend-specific
// capability (only pgstore implements it today). Callers type-assert a Store to
// HistoryReader and degrade gracefully when the assertion fails.
type HistoryReader interface {
	// ListVersions returns the version timeline for an entity id, oldest
	// first, walking rename lineage so a renamed entity's pre-rename history
	// is included. Returns an empty slice (not an error) when the id has no
	// history. The id may name a live or an already-deleted entity.
	ListVersions(ctx context.Context, id string) ([]VersionMeta, error)

	// GetVersion returns the full snapshot for a specific 1-based version
	// ordinal in the entity's lineage. Returns ErrNotFound if the id has no
	// such version.
	GetVersion(ctx context.Context, id string, version int) (*VersionSnapshot, error)
}

// --- Relation versioning (TKT-92JL8P) ---
//
// Relation versioning mirrors entity versioning but is a SEPARATE optional
// capability: the DTOs and the RelationHistoryReader / RelationVersionWriter
// interfaces are distinct from the entity ones, and consumers type-assert them
// independently. This keeps each optional interface narrow (a store may support
// entity history without relation history, or vice versa) and keeps relation
// methods off the entity HistoryReader/VersionWriter surface. See the ticket's
// design notes for why relations need a surrogate lineage id (they have no
// stable key — the composite (from,type,to) mutates on endpoint rename).

// RelationVersionMeta is a single row of a relation's version timeline, without
// the snapshot body/properties. From/Type/To are the composite AS-OF this
// version; PrevFrom/PrevTo are set only for VersionOpRename (the pre-rename
// endpoints). Version is the 1-based ordinal within the relation's lineage
// (keyed by the surrogate rel_record_id), computed at read time, newest last.
type RelationVersionMeta struct {
	Version       int
	Op            VersionOp
	From          string
	Type          string
	To            string
	PrevFrom      string // set only for VersionOpRename
	PrevTo        string // set only for VersionOpRename
	ContentHash   string
	SchemaHash    string
	PrincipalUser string
	PrincipalTool string
	TriggeredBy   string
	CreatedAt     time.Time
}

// RelationVersionSnapshot is a full captured relation version: its metadata plus
// the relation content and properties as they were, and the render-schema
// projection (as stored JSON) the snapshot was taken under.
type RelationVersionSnapshot struct {
	RelationVersionMeta
	Content    string
	Properties map[string]any
	Projection []byte // the schema_versions.projection JSON for SchemaHash
}

// RelationLifetime summarizes one past lifetime of a relation's composite key —
// one stitched-history lineage whose FINAL version row still carries this
// (from,type,to). A key that was deleted-and-recreated has multiple lifetimes
// (each recreate mints a fresh rel_record_id); a key with a single live-or-
// deleted history has one. Lifetime is a 1-based ordinal, 1 = NEWEST, assigned
// within one [RelationHistoryReader.ListRelationLifetimes] response (response-
// local — a concurrent delete can shift ordinals between calls, so the durable
// handle for addressing a specific lifetime is RecordID, not Lifetime).
type RelationLifetime struct {
	Lifetime     int       // 1-based ordinal, 1 = newest
	RecordID     int64     // durable opaque handle (the stitched-head rel_record_id)
	VersionCount int       // number of version rows across the stitched lineage
	FirstSeen    time.Time // min(created_at) across the lineage
	LastSeen     time.Time // max(created_at) across the lineage
	Live         bool      // this lineage is the current live relations-row id
	FinalOp      VersionOp // op of the newest row (delete = ended; else still live/renamed)
}

// RelationHistoryQuery addresses a relation's history by composite key, optionally
// selecting a specific past lifetime. RecordID == 0 selects the NEWEST lifetime
// (byte-for-byte the pre-lifetime-selection behavior); a non-zero RecordID
// selects that specific lineage and MUST be one of the key's lifetimes (see
// [RelationHistoryReader.ListRelationLifetimes]) — the store returns ErrNotFound
// otherwise, so the composite key remains the authorization boundary and RecordID
// only disambiguates within it.
type RelationHistoryQuery struct {
	From     string
	Type     string
	To       string
	RecordID int64 // 0 = newest lifetime
}

// RelationVersionInput is one relation version to persist via
// [RelationVersionWriter]. RecordID is the surrogate lineage id read off the
// live relations row (0 is invalid — the caller must supply the row's
// rel_record_id). PrevFrom/PrevTo are set only for VersionOpRename. Attribution
// arrives here, populated from ctx at the boundary — the store learns the
// Principal by no other route.
type RelationVersionInput struct {
	RecordID      int64
	Op            VersionOp
	From          string
	Type          string
	To            string
	PrevFrom      string
	PrevTo        string
	Content       string
	Properties    map[string]any
	SchemaHash    string
	Projection    []byte
	PrincipalUser string
	PrincipalTool string
	TriggeredBy   string
}

// RelationVersionWriter persists a captured relation version. An optional,
// backend-specific capability (pgstore only), type-asserted independently of the
// entity VersionWriter. The entitymanager's synchronous version hook dispatches
// delete/rename captures here via a wiring-supplied adapter.
type RelationVersionWriter interface {
	// WriteRelationVersion persists one relation_versions row. Best-effort from
	// the caller's perspective (the entitymanager logs and swallows the error),
	// but the implementation should still return a real error for diagnosis.
	WriteRelationVersion(ctx context.Context, in RelationVersionInput) error
}

// RelationHistoryReader reads a relation's captured version history. Optional,
// backend-specific (pgstore only), type-asserted independently of the entity
// HistoryReader. A relation is addressed by its current (or last-known, for a
// deleted relation) composite key; the reader resolves that to a surrogate
// rel_record_id lineage internally.
type RelationHistoryReader interface {
	// ListRelationVersions returns the version timeline for the lifetime the query
	// selects, oldest first. Returns an empty slice (not an error) when the key has
	// no history. The key may name a live or an already-deleted relation. With
	// RecordID == 0 it returns the CURRENT (or most recent) lifetime for the key,
	// not merged across a delete boundary — a re-created (from,type,to) gets a
	// fresh rel_record_id. A non-zero RecordID selects a specific past lifetime
	// (see ListRelationLifetimes) and yields ErrNotFound if it is not a lifetime of
	// this key.
	ListRelationVersions(ctx context.Context, q RelationHistoryQuery) ([]RelationVersionMeta, error)

	// GetRelationVersion returns the full snapshot for a 1-based version ordinal in
	// the lifetime the query selects. Returns ErrNotFound if the key/lifetime has
	// no such version (or the RecordID is not a lifetime of the key).
	GetRelationVersion(ctx context.Context, q RelationHistoryQuery, version int) (*RelationVersionSnapshot, error)

	// ListRelationLifetimes enumerates every past lifetime of a relation's
	// composite key, newest-first (Lifetime 1 = newest). Multiple entries mean the
	// key was deleted-and-recreated: this is how a caller discovers that older
	// deleted lifetimes exist and obtains the RecordID handle to read one. Returns
	// an empty slice for an unknown key.
	ListRelationLifetimes(ctx context.Context, from, relType, to string) ([]RelationLifetime, error)
}

// --- Version purge (TKT-BW6UUL) ---
//
// Purge HARD-DELETES version snapshot rows — the deliberate, audited exception
// to the append-only history model, for compliance redaction (PII / rotated
// secret / GDPR erasure). It is an OPTIONAL, backend-specific capability
// (pgstore only), type-asserted independently of the reader/writer capabilities.
// Separate entity (VersionPurger) and relation (RelationVersionPurger)
// capabilities, one method each. See the ticket + design-review responses for
// the guardrails the implementation MUST enforce (they are load-bearing, not
// optional): mutual exclusion with the reconciliation sweep, refuse-when-live,
// non-rename-rows-only, fenced-lineage --all.

// PurgeSelector chooses which version row(s) in a lineage a purge targets.
// Exactly one of Vseq / ContentHash / All must be set. Vseq and ContentHash are
// STABLE handles (unlike the read-time 1-based ordinal, which renumbers when a
// row is purged) so an operator purges exactly the row they meant even under a
// concurrent capture.
type PurgeSelector struct {
	Vseq        int64  // purge the single row with this vseq (0 = unset)
	ContentHash string // purge every row in the lineage with this content_hash (GDPR "erase this value everywhere")
	All         bool   // purge the entire fenced lineage
}

// VersionPurgeRequest is one entity-version purge. Target is the entity id whose
// lineage is addressed. Reason is a required operator-supplied justification
// recorded in the audit trail (the one record that survives a purge). ForceLive
// overrides the refuse-when-a-live-row-exists guard by writing a no-content
// purge tombstone the sweep respects (see VersionOpPurge). DryRun resolves and
// returns the target rows WITHOUT deleting. Attribution arrives here from ctx at
// the boundary — the store never learns the principal by another route.
type VersionPurgeRequest struct {
	EntityID      string
	Selector      PurgeSelector
	Reason        string
	ForceLive     bool
	DryRun        bool
	PrincipalUser string
	PrincipalTool string
}

// RelationVersionPurgeRequest is the relation analog, addressing a relation by
// its composite key.
type RelationVersionPurgeRequest struct {
	From, Type, To string
	Selector       PurgeSelector
	// RecordID selects which lifetime of a reused key to purge (0 = newest). A
	// key that was deleted-and-recreated has multiple lifetimes; purging without a
	// selector would silently erase only the newest and leave older lifetimes'
	// content behind — a false compliance guarantee. So a multi-lifetime key with
	// RecordID == 0 and AllLifetimes == false is REFUSED (see PurgeResult).
	RecordID int64
	// AllLifetimes purges every lifetime of the key (each fenced lineage), for a
	// complete erasure of a reused key. Mutually exclusive with RecordID.
	AllLifetimes  bool
	Reason        string
	ForceLive     bool
	DryRun        bool
	PrincipalUser string
	PrincipalTool string
}

// PurgeTarget is one version row a purge would delete (or, in DryRun, would
// delete): enough to show the operator and audit the action WITHOUT the snapshot
// content (which must never be echoed — echoing it would defeat the purge).
type PurgeTarget struct {
	Vseq        int64
	Op          VersionOp
	ContentHash string
	CreatedAt   time.Time
	IsRename    bool // a rename row is REFUSED in v1 (purging it orphans lineage)
}

// PurgeResult reports what a purge did (or, in DryRun, would do). Targets is the
// resolved set. Purged is how many rows were actually deleted (0 on DryRun).
// LiveRowExists / RenameInTargets flag the two refuse conditions so a caller can
// render the reason without re-querying.
type PurgeResult struct {
	Targets          []PurgeTarget
	Purged           int
	LiveRowExists    bool
	RenameInTargets  bool
	TombstoneWritten bool
	// MultiLifetimeRefused is set (with Purged == 0) when a relation purge names a
	// key that has more than one lifetime but no lifetime selector (RecordID /
	// AllLifetimes) — the caller must choose, so nothing is erased. LifetimeCount
	// carries how many exist, for the operator message.
	MultiLifetimeRefused bool
	LifetimeCount        int
}

// VersionPurger hard-deletes entity version snapshot rows. Optional,
// backend-specific (pgstore only), type-asserted independently.
type VersionPurger interface {
	// PurgeVersions resolves the request's target rows and, unless DryRun,
	// deletes them — under mutual exclusion with the reconciliation sweep. It
	// REFUSES (deleting nothing, PurgeResult flags the reason) when the target
	// set contains a rename row, or when a live row still holds the content and
	// ForceLive is not set. Returns the resolved/purged set for audit + display.
	PurgeVersions(ctx context.Context, req VersionPurgeRequest) (*PurgeResult, error)
}

// RelationVersionPurger is the relation analog.
type RelationVersionPurger interface {
	PurgeRelationVersions(ctx context.Context, req RelationVersionPurgeRequest) (*PurgeResult, error)
}

// VersionService is the umbrella for a backend's full content-versioning surface:
// entity + relation history reads, synchronous version writes, and purge. It is a
// SEPARATE concern from [Store] (a store just stores) that a backend supplies as
// its own injected service — or leaves absent (nil) where versioning isn't
// provided (the filesystem build uses git instead). pgstore's *VersionStore
// implements it.
//
// This umbrella is a WIRING vehicle only: the composition root uses it as the
// nil-able field type it threads through the service bundles. Consumers still
// bind the NARROW sub-interface they actually use at the call site
// (a history command takes [RelationHistoryReader]; a recorder takes
// [RelationVersionWriter]) — the umbrella is never a parameter to a handler or
// command. It groups one cohesive concern (all version I/O over one connection),
// not a cross-subsystem service locator.
type VersionService interface {
	HistoryReader
	VersionWriter
	RelationHistoryReader
	RelationVersionWriter
	VersionPurger
	RelationVersionPurger
}

// ProjectionProvider yields the current render-schema projection (its content
// hash and its JSON form) for stamping onto swept create/update versions.
//
// Supplied by the wiring layer, which holds the metamodel; a store is
// metamodel-agnostic and must stay that way. Called once per sweep tick rather
// than per entity, so an edit to the metamodel is picked up on the next tick.
//
// Lives here rather than in a backend because it is the CONTRACT for an
// optional capability, like [VersionService] and [DerivedObjectSpec] above: a
// backend that implements versioning must be able to accept one without
// importing whichever backend happened to define it first (TKT-L3FNEN).
type ProjectionProvider interface {
	Projection() (hash string, projectionJSON []byte)
}

// SweepConfig tunes a backend's version-reconciliation sweep. Zero values fall
// back to the implementation's defaults, so the zero SweepConfig is valid and
// means "use production cadence".
//
// The fields describe INTENT — how often to look, how settled a record must be,
// when to give up waiting for it to settle, how much to do at once — not any
// one backend's mechanism, which is why they are expressible here.
type SweepConfig struct {
	// Interval is how often a tick runs.
	Interval time.Duration
	// Idle is how long an entity must be un-touched (updated_at older than
	// now-Idle) before its settled state is snapshotted — the debounce.
	Idle time.Duration
	// MaxStaleness forces a snapshot of a continuously-edited entity whose
	// latest version is older than this, even if it never settles.
	MaxStaleness time.Duration
	// Batch caps how many entities one tick processes, so a bulk-import burst
	// drains across ticks instead of running unboundedly.
	Batch int
}

// VersionSweeper is a store that runs its own debounced reconciliation sweep to
// capture create/update versions.
//
// Optional, type-asserted at the wiring site like [Formatter] and
// [HistoryReader] — not part of [Store].
type VersionSweeper interface {
	StartVersionSweep(provider ProjectionProvider, cfg SweepConfig)
}

// VersionServiceProvider is a store that can hand out a [VersionService]
// sharing its own connection or handle.
//
// Nil: an implementation MAY return nil (a partially-initialized backend, say).
// Callers must treat a nil return as "no versioning" rather than boxing it into
// the interface — a nil pointer inside a non-nil interface passes every
// downstream nil-check and panics at write time instead.
type VersionServiceProvider interface {
	VersionStore() VersionService
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
