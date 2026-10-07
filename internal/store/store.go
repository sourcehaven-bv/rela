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
	// ErrAttachmentTooLarge is returned by AttachFamilyFile when the supplied
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
// every backend's AttachFamilyFile applies.
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
//
// An iterator MUST NOT hold a backend resource (a pooled connection, an
// open cursor, a lock) while it yields. Callers make store calls inside the
// loop body: visibility redaction checks ACL edges for every row. A backend
// that held its connection across yield would need a second one per
// concurrent iteration and deadlock once iterations outnumber its pool
// (BUG-9TGOH1). Read a batch, release it, then yield. The same holds for any
// callback a read method invokes. Pinned by storetest's IteratorNesting
// suite.
type EntityReader interface {
	// GetEntity returns the one row ref addresses: the face ref.Face of the
	// family ref.ID. Returns ErrNotFound if that row does not exist,
	// including when other faces of the family do.
	//
	// The zero Face is a real coordinate, not a default: it is the implicit
	// face of a faceless type. A faced type stores no row there
	// (BUG-HC6I2T), so Ref{ID: id} on a faced family is ErrNotFound. The
	// store cannot tell the two cases apart, because it holds no metamodel;
	// the caller names the face it means (DEC-NPZICR).
	//
	// GetEntity does no parsing. ref.ID is a bare id: Ref{ID: "X@draft"}
	// and the zero Ref are ErrNotFound on every backend, as is a malformed
	// face, never a path or I/O error.
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)

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
	// given prefix (e.g. "FEAT" → 42 if FEAT-042 is the highest), over
	// every face of every family: an id stored only at named faces is
	// taken. Returns 0 if no entities with the prefix exist.
	HighestID(ctx context.Context, prefix string) (int, error)
}

// EntityQuery filters entity listings.
type EntityQuery struct {
	Type   string   // filter by entity type (empty = all)
	IDs    []string // filter to specific IDs (empty = all)
	Cursor string   // pagination cursor from a previous page (empty = start); ignored by ListEntities
	Limit  int      // max entities per page (0 = no limit); ignored by ListEntities

	// Faces is the CALLER's face selection: [InWorld], [AllFaces] or
	// [AtFaces]. It is required; the zero value is [ErrInvalidQuery] on every
	// backend (TKT-KQXVF7), because a default is how faced types went
	// missing. It is caller-owned; [EntityQuery.FaceIn] is the ACL's.
	Faces FaceSelection

	// FaceIn narrows the result to these content states — a SET filter,
	// semantically distinct from an InWorld selection's ranked chain
	// (TKT-O7R2A1). It is AUTHORIZATION-owned: a role's face grants compile
	// to it. [EntityQuery.Faces] is the caller's selection it composes with.
	//
	// Nil means every face, which is what every pre-faces caller passes, so
	// the historical query shape is untouched.
	//
	// Composes WITH every selection rather than replacing it: under InWorld
	// the world picks each entity's prime by rank, and this narrows the
	// CANDIDATES that ranking runs over. Applied before the rank, so an
	// entity whose top-choice face is excluded falls through to the next
	// candidate the caller may see rather than vanishing — the same "select,
	// and fall back" the chain already means. Under AllFaces and AtFaces it
	// is intersected with the rows the selection admits.
	//
	// A backend that ignores it FAILS OPEN, so the conformance suite pins it.
	FaceIn []entity.Face
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

	// UpdateEntity persists changes to an existing entity, unconditionally:
	// it is last-write-wins and will overwrite a concurrent writer's changes.
	// Returns ErrNotFound if the entity does not exist.
	//
	// Callers doing read-modify-write want [EntityWriter.UpdateEntityIf].
	UpdateEntity(ctx context.Context, e *entity.Entity) error

	// UpdateEntityIf is UpdateEntity with a compare-and-swap precondition:
	// the write applies only if the stored record still matches
	// cond.ExpectedVersion (see [VersionOf]). It returns the version the
	// entity has AFTER a successful write, so a caller looping over several
	// conditional writes never needs to re-read to get its next token.
	//
	// This is the primitive for read-modify-write. It replaces the
	// check-then-write shape, which is safe only under a process-local
	// mutex and therefore not safe at all in a deployment running several
	// processes against one database (docs/postgres-backend.md).
	//
	// Errors:
	//   - ErrNotFound if the entity does not exist. A caller that expected a
	//     specific version and finds the row deleted gets this, NOT a
	//     conflict — the distinction matters because retrying cannot help.
	//   - *VersionConflictError if the row exists but has moved on. NOTHING
	//     was written. The error carries the current version, so the caller
	//     can re-read, recompute, and retry with a bounded loop.
	//
	// A zero cond makes this identical to UpdateEntity. That is a deliberate
	// convenience for generic code, not an invitation — a caller that means
	// to be unconditional should say so by calling UpdateEntity.
	//
	// **Relationship to [Transactor.Tx]** (DEC-8UIL0): the precondition is
	// evaluated identically inside and outside a Tx. Inside a pgstore
	// transaction it is largely redundant, since the transaction and its
	// advisory lock already serialize writers — but not entirely: it still
	// catches an expectation formed BEFORE the Tx opened. Keeping the check
	// unconditional means one documented contract, and no caller has to
	// reason about whether it happens to be inside a transaction.
	UpdateEntityIf(
		ctx context.Context, e *entity.Entity, cond UpdateCondition,
	) (EntityVersion, error)

	// DeleteFamily removes every face of id and, with cascade, every
	// relation incident to the family on both sides and every tail.
	// Returns ErrNotFound if no face of id exists. Without cascade, a family
	// that still has an incident relation is refused with ErrHasRelations.
	//
	// A non-nil *DeleteResult MAY accompany a non-nil error — the one place
	// this package departs from "error means ignore the value". A
	// non-transactional backend (fsstore: Tx is a write mutex with no
	// rollback) can fail partway through a cascade with some relation files
	// already off disk. It then returns what it genuinely removed, so the
	// caller's audit log can reflect the real state rather than denying a
	// deletion that happened (issue #929). Transactional backends
	// (pgstore, sqlitestore) roll back and return nil.
	//
	// A partial result is NOT success: the caller must still return the
	// error. It lists only what really happened — DeletedEntities is
	// populated if and only if the entity file was removed, so on a partial
	// cascade (which aborts before or at that removal) it is empty.
	DeleteFamily(ctx context.Context, id string, cascade bool) (*DeleteResult, error)

	// DeleteFace removes the one face row ref addresses, leaving the rest of
	// the family standing (TKT-C1XUA8). Ref{ID: id} is a valid address: the
	// implicit face of a faceless type.
	//
	// This is NOT a narrower DeleteFamily. Deleting a face removes one row
	// and only the edges that belong to it:
	//
	//   - OUTGOING edges whose tail is this face go WITH it. They
	//     were written against this face and nothing else can own them.
	//   - INCOMING edges SURVIVE while any face remains. Heads are
	//     entity-level (design doc §2.3), so an inbound edge points at the
	//     ENTITY, not at one of its faces — deleting them would let removing
	//     a draft silently cut links that unrelated entities hold on the
	//     published face.
	//
	// Deleting the LAST face leaves no entity, so it also removes every
	// relation still incident to the family: inbound edges and edges on any
	// other tail (RR-2466U1). Otherwise they would point at, or start from,
	// an entity that no longer exists. The last-face delete therefore
	// removes exactly what DeleteFamily(id, true) removes, and
	// DeleteResult.DeletedRelations lists every one of them, so the caller
	// can authorize, version and audit them. The caller that must not cut
	// those edges checks the family size first.
	//
	// Deleting the implicit face while named faces remain is ALLOWED. It
	// was once refused (a family was required to keep a default row), but
	// BUG-HC6I2T removed that invariant: a type declaring `faces:` stores
	// nothing at the zero coordinate, so the refusal would have made the
	// flat→faced migration impossible. It is what migrate_face and
	// `rela migrate adopt-face` do on every row they move, and
	// TestFaces_RowCanLeaveTheZeroCoordinate pins it.
	//
	// Returns ErrNotFound if that face does not exist.
	DeleteFace(ctx context.Context, ref entity.Ref) (*DeleteResult, error)

	// RenameFamily changes the id of every face of oldID, and every
	// relation endpoint and tail that names it, atomically.
	// Returns ErrNotFound if no face of oldID exists.
	// Returns ErrConflict if any face of newID already exists.
	RenameFamily(ctx context.Context, oldID, newID string) (*RenameResult, error)
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
	// GetRelation returns the relation with key k. The tail (k.FromFace) is
	// part of the address: the zero face reads the identity-scoped or
	// faceless-source edge, never "any tail" (TKT-KQXVF7).
	// Returns ErrNotFound if the relation does not exist.
	GetRelation(ctx context.Context, k entity.RelationKey) (*entity.Relation, error)

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

	// EntityIDs is the plural of EntityID: the edge's endpoint (per
	// Direction, exactly as for EntityID) must be one of these ids. It
	// exists so a page of rows loads its edges in ONE query instead of one
	// per row (TKT-1U8XYN); it composes with Type and FromFace like EntityID
	// does. Nil means unfiltered; an empty, non-nil slice matches nothing —
	// a caller that computed "no ids" must get no edges, not all of them.
	// When both EntityID and EntityIDs are set an edge must satisfy both.
	//
	// A backend must accept a batch of ANY size in one call: callers hand
	// it a whole page or a whole subtree and do not chunk. A backend with a
	// bind-parameter budget has to pack the batch itself (sqlitestore sends
	// one JSON array). Pinned by storetest's ListEntityIDsLargeBatch.
	EntityIDs []string
	Cursor    string // pagination cursor from a previous page (empty = start); ignored by ListRelations
	Limit     int    // max relations per page (0 = no limit); ignored by ListRelations

	// FromFace filters on the state-specific tail of an edge
	// (TKT-DOFYR1). nil — the zero value — leaves the tail UNFILTERED:
	// edges from every state plus identity edges all match, which is
	// exactly today's behavior for faceless projects and the compat
	// story for every existing query site. Non-nil matches the tail
	// face by equality (the zero Face matches default-tail edges
	// only). Stores compare, never inspect — see entity.Face.
	FromFace *entity.Face
}

// Direction constrains relation queries to a specific direction.
type Direction int

const (
	DirectionBoth     Direction = iota // match both outgoing and incoming
	DirectionOutgoing                  // match only outgoing relations
	DirectionIncoming                  // match only incoming relations
)

// RelationWriter provides write access to relations.
//
// Every method takes the relation's full [entity.RelationKey], tail
// included. The tail is part of a relation's IDENTITY, not a filter
// (TKT-C1XUA8): two edges on one triple with different tails are two
// relations. The key is the only place the tail lives, so an address and a
// payload can never disagree about it (BUG-64MU2Q, TKT-KQXVF7).
type RelationWriter interface {
	// CreateRelation persists a new relation at k. data may be nil.
	// Returns ErrConflict if the relation already exists.
	CreateRelation(ctx context.Context, k entity.RelationKey, data *RelationData) (*entity.Relation, error)

	// UpdateRelation replaces the data of the relation at k.
	// Returns ErrNotFound if no edge with that exact key exists.
	UpdateRelation(ctx context.Context, k entity.RelationKey, data RelationData) (*entity.Relation, error)

	// DeleteRelation removes the relation at k.
	// Returns ErrNotFound if no edge with that exact key exists.
	DeleteRelation(ctx context.Context, k entity.RelationKey) error
}

// RelationData holds optional properties and content for a relation. The
// relation's address, tail included, is the [entity.RelationKey] passed
// beside it.
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

// AttachmentManager provides file attachment operations. Attachment bytes
// belong to the FAMILY, not to one face: every face of an id shares one
// byte store, and a face's property value only names the files it uses
// (TKT-KQXVF7, Stage 1 design section 4). The method names say so.
//
// A property can hold multiple attachments, each keyed by its (normalized)
// file name — so reads and deletes target a specific (entityID, property,
// fileName). AttachFamilyFile appends; it does not overwrite other files on
// the property. Enforcing a per-property cap (the metamodel `max`) and
// replace-at-1 semantics is the write path's job, not the store's.
type AttachmentManager interface {
	// AttachFamilyFile stores bytes for the family entityID. Returns
	// ErrNotFound when no face of entityID exists, and accepts the write
	// when any face does.
	AttachFamilyFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error
	ReadFamilyAttachment(ctx context.Context, entityID, property, fileName string) (io.ReadCloser, error)
	DeleteFamilyAttachment(ctx context.Context, entityID, property, fileName string) error
	ListFamilyAttachments(ctx context.Context, entityID string) ([]AttachmentInfo, error)
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
	ID   string
	Type string

	// Face identifies the content state this header describes; zero =
	// default state (TKT-DOFYR1). Populated so AllFaces header scans can
	// tell a family's rows apart.
	Face entity.Face

	Properties map[string]any
	UpdatedAt  time.Time

	// Redacted mirrors [entity.Entity.Redacted]: the properties withheld
	// from the reading principal by field-level ACL. Carried so a gated
	// header read reports redaction exactly like a gated entity read —
	// a header must never look MORE complete than the entity it projects.
	Redacted []string

	// Inaccessible mirrors [entity.Entity.Inaccessible]: the fields a
	// storage-level failure (an undecryptable git-crypt file) withheld. A
	// list rendered from headers must show the same lock a list rendered
	// from entities did (TKT-1U8XYN); dropping it here silently un-locked
	// every cell of an encrypted row.
	Inaccessible []entity.InaccessibleField
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
		ID:           e.ID,
		Type:         e.Type,
		Face:         e.Face,
		Properties:   e.Properties,
		UpdatedAt:    e.UpdatedAt,
		Redacted:     e.Redacted,
		Inaccessible: e.Inaccessible,
	}
}

// EntityLister is the one read [ListEntityHeaders] needs. Every
// [EntityReader] satisfies it; it exists so a consumer holding a narrower
// read surface than a whole reader can still list headers.
type EntityLister interface {
	ListEntities(ctx context.Context, q EntityQuery) iter.Seq2[*entity.Entity, error]
}

// ListEntityHeaders lists content-free entity headers from any reader.
//
// Uses the reader's native [HeaderReader] when it has one, so the body never
// leaves the backend; otherwise falls back to [EntityLister.ListEntities] and
// projects each row as it is yielded. The fallback bounds RETENTION (rows are
// converted and released one at a time, never accumulated) but not transfer —
// a backend without the capability still reads bodies off disk or the wire.
// Do not describe the fallback as bounding I/O.
func ListEntityHeaders(
	ctx context.Context, r EntityLister, q EntityQuery,
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

// BulkMigrator performs whole-table migration rewrites natively (TKT-HH7PKJ).
//
// OPTIONAL capability, type-asserted at the call site like [HeaderReader] and
// [Formatter] — not part of [Store]. A backend that can express a rewrite as a
// set operation implements it; the rest are served by the generic fallback
// beside each dispatcher, which reads, rewrites and writes row by row.
//
// Callers should use the package-level dispatcher ([SwapRelationEndpoints])
// rather than asserting themselves, so the fallback stays a detail of this
// package.
//
// # Why this is a store concern
//
// The alternative is a loop above the store, and for an endpoint swap that loop
// must be create-then-delete, because a relation's endpoints are its ADDRESS
// (see [RelationWriter.UpdateRelation], which mutates only data). That shape
// carries hazards the set operation does not have: a self-edge whose reversed
// triple collides with itself, a both-directions pair that collides with its
// partner, and — on the versioning backends — a fresh version lineage per edge,
// since delete+create mints a new rel_record_id. An in-place re-key keeps the
// row, so it keeps its identity and its history; this is the same property that
// made [EntityWriter.RenameFamily] atomic.
//
// # Vocabulary
//
// Members take store-level arguments only (a relation type name, a property
// key) and never an application type: a store must not depend on the metamodel
// (arch-lint enforces it), so every schema-shaped decision — whether a relation
// type is content-scoped, whether its endpoints overlap — stays ABOVE this
// seam, in the caller. A member is a mechanical rewrite of rows the caller has
// already decided are safe to rewrite.
//
// Today it has one member. RenameRelationType (a rel_type column update),
// RenameProperty and MapValues (both jsonb surgery, which no backend does
// in-place yet) are the intended next ones; the interface is named for the job
// rather than the method so they can join it without a rename.
type BulkMigrator interface {
	// SwapRelationEndpoints exchanges from and to on every relation of
	// relType, returning the number of rows rewritten.
	//
	// The caller guarantees the rewrite is safe for this type: no edge carries
	// a tail face (a head has no face slot, so a reversed state-tailed edge is
	// unrepresentable), and the type's endpoint lists do not overlap. An
	// implementation performs the rewrite and does not re-derive those.
	//
	// A collision — two edges that would swap onto the same triple, or a
	// self-edge in a backend that cannot rewrite a row onto itself — must
	// fail with an error and leave the data unchanged, never half-rewritten.
	SwapRelationEndpoints(ctx context.Context, relType string) (int, error)
}

// SwapRelationEndpoints exchanges from and to on every relation of relType.
//
// Uses the store's native [BulkMigrator] when it has one — pgstore and
// sqlitestore rewrite the table in one statement — and otherwise falls back to
// a read-rewrite-write loop through the public writer API.
//
// The two paths are held to ONE contract by storetest: same counts, same
// refusals, same surviving data. The fallback differs in exactly one
// observable way, and only on a backend with version history: it is
// delete-then-create, so each edge starts a new lineage. The backends that
// have history (pg, sqlite) are precisely the ones implementing the native
// path, so in practice no deployment pays that cost.
//
// Nil: s is required.
func SwapRelationEndpoints(ctx context.Context, s Store, relType string) (int, error) {
	// Checked on BOTH paths, not just the fallback: a native backend discovers
	// a collision only by attempting the write, so without this a pg/sqlite
	// caller gets a different error (and a different moment of failure) than an
	// fs caller for the same data. The native statement's unique constraint
	// remains the backstop.
	if _, err := CheckSwapRelationEndpoints(ctx, s, relType); err != nil {
		return 0, err
	}
	if bm, ok := s.(BulkMigrator); ok {
		return bm.SwapRelationEndpoints(ctx, relType)
	}
	return swapRelationEndpointsFallback(ctx, s, relType)
}

// CheckSwapRelationEndpoints reports whether every edge of relType can be
// reversed, without writing anything.
//
// Exported because a dry-run needs the same answer an apply gives, and the
// NATIVE backends have no read-only path to their own unique constraint — they
// learn about a collision only by attempting the write. A caller that previews
// a reversal calls this; [SwapRelationEndpoints] calls it too, so the two can
// never disagree about what is legal.
//
// Returns the number of edges a reversal would rewrite (self-edges excluded:
// reversing one is a no-op).
func CheckSwapRelationEndpoints(ctx context.Context, s Store, relType string) (int, error) {
	var rels []*entity.Relation
	for r, err := range s.ListRelations(ctx, RelationQuery{Type: relType}) {
		if err != nil {
			return 0, err
		}
		rels = append(rels, r)
	}
	if err := checkSwappable(rels, relType); err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rels {
		if r.From != r.To {
			n++
		}
	}
	return n, nil
}

// checkSwappable refuses a set of edges that cannot be reversed as a whole.
//
// Two refusals, both computed before any write so they hold identically on the
// native and fallback paths:
//
//   - A state-tailed edge. The tail is part of a relation's IDENTITY
//     ([entity.Relation.Key] serializes it into the FROM slot) and a head has
//     no face slot at all, so a reversed state-tailed edge cannot be
//     represented and two edges differing only by their tail would merge. The
//     BulkMigrator contract says the caller guarantees this; the guarantee is
//     ASSERTED here rather than assumed, because a store that depends on a
//     precondition for correctness should not be the only thing that cannot
//     see it violated.
//   - A pair that would swap onto each other's triple. The mirror key carries
//     the tail face, so it compares like with like — a face-blind mirror would
//     both miss real collisions and invent false ones the moment tailed edges
//     were admitted.
func checkSwappable(rels []*entity.Relation, relType string) error {
	existing := make(map[string]bool, len(rels))
	for _, r := range rels {
		if !r.FromFace.IsImplicit() {
			return fmt.Errorf(
				"%w: %s is tailed at face %q — a reversed state-tailed edge has nowhere to put "+
					"the face, since a head is entity-level by construction",
				ErrInvalidQuery, r.Key(), r.FromFace)
		}
		existing[r.Key()] = true
	}
	for _, r := range rels {
		// A self-edge reverses onto itself. The native path rewrites the row
		// in place (a no-op); create-then-delete would create nothing and then
		// delete the only copy, so it is skipped rather than "handled".
		if r.From == r.To {
			continue
		}
		mirror := &entity.Relation{From: r.To, FromFace: r.FromFace, Type: r.Type, To: r.From}
		if existing[mirror.Key()] {
			return fmt.Errorf(
				"%w: %s--%s--%s would swap onto %s--%s--%s, which already exists — "+
					"reversing this type would merge two distinct edges",
				ErrConflict, r.From, relType, r.To, mirror.From, relType, mirror.To)
		}
	}
	return nil
}

// swapRelationEndpointsFallback rewrites the edges one at a time.
//
// Collision detection is a PRE-FLIGHT pass over the whole set, not a per-edge
// check: the native path gets it from a unique constraint evaluated against the
// final state, and a loop that discovered a collision halfway would already
// have destroyed the edges it rewrote. Refusing before the first write is what
// makes the two paths agree, and what makes a dry-run above this layer honest.
func swapRelationEndpointsFallback(ctx context.Context, s Store, relType string) (int, error) {
	var rels []*entity.Relation
	for r, err := range s.ListRelations(ctx, RelationQuery{Type: relType}) {
		if err != nil {
			return 0, err
		}
		rels = append(rels, r)
	}

	if err := checkSwappable(rels, relType); err != nil {
		return 0, err
	}

	n := 0
	for _, r := range rels {
		if r.From == r.To {
			continue
		}
		data := &RelationData{Properties: r.Properties, Content: r.Content}
		mirror := entity.RelationKey{From: r.To, FromFace: r.FromFace, Type: r.Type, To: r.From}
		if _, err := s.CreateRelation(ctx, mirror, data); err != nil {
			return n, fmt.Errorf("swap %s--%s--%s: %w", r.From, r.Type, r.To, err)
		}
		// The full key, tail included: dropping the tail would delete a
		// DIFFERENT edge and report success. The caller has refused tailed
		// edges already; keying on Identity means a future relaxation cannot
		// reintroduce that.
		if err := s.DeleteRelation(ctx, r.Identity()); err != nil {
			return n, fmt.Errorf("swap %s--%s--%s: remove the original: %w", r.From, r.Type, r.To, err)
		}
		n++
	}
	return n, nil
}

// Formatter checks whether an entity/relation's persisted representation
// is up to date with its canonical format. Optionally applies the format.
//
// This is NOT part of the Store interface — formatting is a persistence-layer
// concern specific to each backend. Stores that have a canonical serialized
// format (markdown files, YAML, etc.) provide their own Formatter.
type Formatter interface {
	// FormatEntity checks whether the persisted form of the row ref
	// addresses differs from its canonical formatted form. If dryRun is false
	// and it differs, the row is rewritten. Returns changed=true if a rewrite
	// was (or would be) needed, and ErrNotFound if the row does not exist.
	FormatEntity(ctx context.Context, ref entity.Ref, dryRun bool) (changed bool, err error)

	// FormatRelation behaves like FormatEntity but for the relation at k.
	FormatRelation(ctx context.Context, k entity.RelationKey, dryRun bool) (changed bool, err error)
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
	Version int
	Op      VersionOp
	PrevID  string // set only for VersionOpRename: the entity's former ID
	Type    string

	// Face is the face this version captured; zero is the implicit face of a
	// faceless type. A lineage is per (id, face), so every row of one
	// timeline carries the face it was read at. Restoring a deleted face
	// recreates it here (TKT-7R0ABK).
	Face entity.Face

	ContentHash   string
	SchemaHash    string
	PrincipalUser string
	PrincipalTool string
	TriggeredBy   string

	// Origin is the provenance of the write this version captured: zero for a
	// direct edit, [OriginCopy] (with its source) for a copy. Deliberately NOT
	// folded into Op — a copy is still a create or an update OF THE TARGET
	// ROW, and both facts are wanted at once ("v3, an update, copied from
	// POL-1@draft"). A sixth VersionOp value would also silently reclassify
	// the write for every existing client that switches on the five.
	Origin Origin

	CreatedAt time.Time
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
	EntityID string

	// Face names the CONTENT STATE this snapshot captures; zero is the
	// default face (TKT-C1XUA8). It participates in the content hash, so two
	// faces holding identical bytes do not dedup against one another.
	Face entity.Face

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

	// Origin is the provenance of the captured write (zero = direct edit).
	// Like the principal fields it is boundary-populated and travels INSIDE
	// this input for a synchronous capture; the sweep reads its own copy off
	// the live row's columns.
	Origin Origin
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
//     scope reconstructible, and from pgstore's writeEntityTombstone it looks
//     like one more column. It is not a schema question. The row would then
//     assert "this deleted subject belonged to that project", so a relational
//     fact about a person SURVIVES their deletion — which inverts what deletion
//     is for and is a GDPR/AVG question before it is a performance one. That is
//     the precondition: answer it, then widen the tombstone, then narrow the
//     watermark. Not in the other order.
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

// HistoryReader reads the captured version history of one face row. Like
// Formatter it is NOT part of the Store interface — content versioning is a
// backend-specific capability (pgstore and sqlitestore implement it). Callers
// type-assert a Store to HistoryReader and degrade gracefully when the
// assertion fails.
//
// A lineage is per (id, face): the history of POL-1@concept never folds in
// POL-1@vastgesteld. Ref{ID: id} reads the implicit face of a faceless type.
// A ref that no row can have (see storeutil.Addressable) has no history: an
// empty timeline and ErrNotFound for every version.
//
// NOTE for read-path callers: a face's history is as sensitive as the face,
// and the face is caller-supplied. A surface that lets a principal name a
// face must authorize that face — the world read grant (TKT-DN37J2), not
// merely the entity's read verdict. The store does not and cannot check it.
type HistoryReader interface {
	// ListVersions returns the version timeline of the face ref addresses,
	// oldest first, walking rename lineage so a renamed entity's
	// pre-rename history is included. Every row carries ref.Face in
	// [VersionMeta.Face]. Returns an empty slice (not an error) when the
	// face has no history. The face may be live or already deleted.
	ListVersions(ctx context.Context, ref entity.Ref) ([]VersionMeta, error)

	// GetVersion returns the full snapshot for a specific 1-based version
	// ordinal in that face's lineage. Returns ErrNotFound if it has no such
	// version.
	GetVersion(ctx context.Context, ref entity.Ref, version int) (*VersionSnapshot, error)
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
	// Key is the relation whose history is read. Its FromFace is part of
	// the key, not a filter over it (TKT-JAROC3): a triple can hold one edge
	// per tail and those are different relations with their own lineages.
	// The zero tail reads the identity-scoped or faceless-source edge's
	// history specifically, never "any tail".
	//
	// A non-zero RecordID selects one lineage of the key, and the store
	// validates it against the whole key, tail included: a RecordID that
	// belongs to a sibling tail answers [ErrNotFound].
	Key entity.RelationKey

	RecordID int64 // 0 = newest lifetime
}

// RelationVersionInput is one relation version to persist via
// [RelationVersionWriter]. RecordID is the surrogate lineage id read off the
// live relations row; 0 asks the store to resolve it from the composite key
// (including FromFace), which is correct for a synchronous capture taken while
// the row still exists. PrevFrom/PrevTo are set only for VersionOpRename.
// Attribution arrives here, populated from ctx at the boundary — the store
// learns the Principal by no other route.
type RelationVersionInput struct {
	RecordID int64

	// Key is the versioned edge AS OF this version, tail included
	// (TKT-C1XUA8). Lineages were already fenced by RecordID; the tail is
	// what lets the rename stitch tell a state-tailed predecessor from a
	// default-tail one holding the same triple.
	Key entity.RelationKey

	Op            VersionOp
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

// RelationRecordIDReader reads the surrogate lineage id (rel_record_id) of a
// live relation row. An optional capability of the stores that version
// relations (pgstore, sqlitestore); a Tx view of those stores has it too.
//
// A caller that deletes an edge inside a Tx and records its delete version
// after the Tx reads the id here first. Once the row is gone, a version
// written with RecordID 0 can no longer resolve the edge's own lineage.
type RelationRecordIDReader interface {
	// RelationRecordID returns the id of the live edge k names, tail
	// included, or ErrNotFound.
	RelationRecordID(ctx context.Context, k entity.RelationKey) (int64, error)
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
	//
	// The key's FromFace scopes the enumeration to ONE tail (TKT-JAROC3).
	// Tails are separate relations with separate lineages, so
	// listing them together would offer a caller asking about the draft edge
	// a handle to the published edge's history — and the two are indis-
	// tinguishable in the response, which carries no face.
	ListRelationLifetimes(ctx context.Context, k entity.RelationKey) ([]RelationLifetime, error)
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

// VersionPurgeRequest is one entity-version purge. Ref is the face whose
// lineage is addressed. Reason is a required operator-supplied justification
// recorded in the audit trail (the one record that survives a purge). ForceLive
// overrides the refuse-when-a-live-row-exists guard by writing a no-content
// purge tombstone the sweep respects (see VersionOpPurge). DryRun resolves and
// returns the target rows WITHOUT deleting. Attribution arrives here from ctx at
// the boundary — the store never learns the principal by another route.
type VersionPurgeRequest struct {
	// Ref names the face whose history is purged; Ref{ID: id} is the
	// implicit face of a faceless type (TKT-C1XUA8). Purge is scoped to one
	// face: each face has its own fenced lineage, and erasing a sibling's
	// history because it shares bytes would destroy records the operator did
	// not ask about.
	Ref entity.Ref

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
	// Key is the relation whose history is purged. Its tail is part of the
	// key, as in [RelationHistoryQuery.Key], so a purge reaches one tail's
	// lineages only and never a sibling tail's (BUG-4SYAA6).
	Key      entity.RelationKey
	Selector PurgeSelector
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

// FaceObserver is the OPTIONAL face-aware extension of [EntityObserver],
// type-asserted by stores the way [Formatter] and [HistoryReader] are.
// An observer that implements it is told which CONTENT STATE a delete
// concerned; one that does not keeps the bare-id contract unchanged.
//
// # Why this could not stay on EntityObserver
//
// [EntityObserver.EntityDelete] takes a bare id, and a bare id is not an
// address once an entity has several faces. Before per-world indexing
// that was harmless because indexes held one document per entity, so the
// stores SUPPRESSED per-face deletes entirely: fsstore and memstore only
// called EntityDelete when the LAST face went away, since notifying on
// any earlier one would de-index an entity that still existed.
//
// A face-keyed index inverts that. It now holds one document per face, so
// it needs the opposite notification — delete exactly the face that went,
// leave the siblings. The bare-id callback cannot express that, and
// widening EntityDelete's signature would break every observer for the
// benefit of the two that index faces.
//
// So the split is by CAPABILITY, and the two callbacks are mutually
// exclusive per delete: a store emits EntityFaceDelete to observers that
// implement this interface, and the legacy last-face-only EntityDelete to
// those that do not. Neither observer sees a notification it cannot act
// on, and no observer sees a delete twice.
//
// Nil: never returned — this is an interface, and a store MUST fall back
// to the plain [EntityObserver] path when the assertion fails.
type FaceObserver interface {
	EntityObserver

	// EntityFaceDelete is called when ONE content state of an entity is
	// removed, whether or not other faces survive. The face addresses
	// the face that went; the zero face is the default state.
	//
	// Deleting a whole entity emits one call PER face, so an observer
	// that removes exactly the named face converges on an empty family
	// without needing to know the family size.
	EntityFaceDelete(id string, p entity.Face) error
}

// Event represents a change that occurred in the store.
type Event struct {
	Op           EventOp
	EntityType   string
	EntityID     string
	RelationType string
	From         string
	To           string

	// Face identifies which content state the event is about; zero =
	// the default state (TKT-DOFYR1). Events fire per state like any
	// write, and observers now receive them: the search indexers key
	// documents per FACE (TKT-9KZGJO), so a state event indexes its own
	// document rather than overwriting the default face.
	Face entity.Face
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
