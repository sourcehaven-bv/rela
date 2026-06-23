package dataentry

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	synctypes "github.com/Sourcehaven-BV/rela/internal/sync"
)

// --- Consumer-side interfaces (declared at the call site per CLAUDE.md) ---

// manifestProvider is the sync manifest source. Only pgstore implements it
// (sync is fs-client ↔ pg-server), so it is wired optionally: nil on the
// fs/memory builds, where the manifest endpoint returns 501.
type manifestProvider interface {
	ManifestSince(ctx context.Context, cursor int64) ([]synctypes.ManifestEntry, error)
}

// syncApplier is the id-preserving, automation-suppressed write path the sync
// push uses. *entitymanager.Manager satisfies it; the EntityManager interface
// deliberately omits these methods (sync is their only consumer).
type syncApplier interface {
	ApplyEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	ApplyRelation(ctx context.Context, r *entity.Relation) (*entity.Relation, error)
}

// syncManifest returns the manifest provider when the store supports it
// (pgstore), else nil. Derived lazily from a.store rather than cached at
// construction, so it stays correct if the store is re-pointed (e.g. test
// rebind). Sync is fs-client ↔ pg-server, so this is nil on fs/memory builds.
func (a *App) syncManifest() manifestProvider {
	if mp, ok := a.store.(manifestProvider); ok {
		return mp
	}
	return nil
}

// syncApplierFor returns the id-preserving applier when the entity manager
// supports it (*entitymanager.Manager), else nil. Derived lazily for the same
// reason as syncManifest.
func (a *App) syncApplierFor() syncApplier {
	if ap, ok := a.entityManager.(syncApplier); ok {
		return ap
	}
	return nil
}

// --- Wire DTOs ---

type syncManifestResponse struct {
	Changes []syncManifestChange `json:"changes"`
	Cursor  string               `json:"cursor"`
}

type syncManifestChange struct {
	Kind    string `json:"kind"` // "e" or "r"
	ID      string `json:"id"`   // entity id, or "from--type--to" for a relation
	Typ     string `json:"typ,omitempty"`
	Deleted bool   `json:"deleted"`
}

// syncEntityBody is the JSON push/fetch payload for an entity.
type syncEntityBody struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
}

// syncRelationBody is the JSON push/fetch payload for a relation.
type syncRelationBody struct {
	From       string         `json:"from"`
	Type       string         `json:"type"`
	To         string         `json:"to"`
	Properties map[string]any `json:"properties,omitempty"`
	Content    string         `json:"content,omitempty"`
}

// registerSyncRoutes mounts the sync API under /api/sync/. See sync.go's
// handlers for the per-route contract. The routes inherit the data-entry
// security middleware EXCEPT the same-origin check, from which /api/sync/ is
// exempted (a non-browser sync client sends no Origin) — see
// middleware_security.go.
func (a *App) registerSyncRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/sync/manifest", a.handleSyncManifest)
	mux.HandleFunc("/api/sync/", a.handleSyncRecord)
}

// handleSyncManifest: GET /api/sync/manifest?cursor=<token>. Returns the changes
// since the cursor and a new cursor (the highest seq seen). The cursor is a
// server-minted token the client stores and echoes back; today it is the seq
// watermark rendered as a decimal string (the client must treat it as opaque
// and not derive meaning from it — the encoding may change). A missing or
// malformed cursor is treated as 0 (full manifest), which is the safe degrade:
// the client re-bootstraps rather than silently skipping changes.
func (a *App) handleSyncManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET", "")
		return
	}
	mp := a.syncManifest()
	if mp == nil {
		writeV1Error(w, r, http.StatusNotImplemented, "sync_unsupported",
			"The sync manifest is only available on the postgres backend", "")
		return
	}

	cursor := parseCursor(r.URL.Query().Get("cursor"))
	entries, err := mp.ManifestSince(r.Context(), cursor)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "manifest_failed", "Failed to read the manifest", "")
		return
	}

	// ACL read gate (RR IB-review #1): the manifest reads entities/relations
	// (and their tombstones) straight from the store, so it must be filtered to
	// the principal's read scope just like every other read path — otherwise
	// any authenticated client learns the full id/relation set, including rows
	// it has no right to read. Denied rows are dropped from Changes; the cursor
	// still advances past them (so the client doesn't re-fetch the same hidden
	// rows forever) — the highest seq is taken over ALL entries, visible or not.
	visible, err := a.filterVisibleManifest(r.Context(), entries)
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	resp := syncManifestResponse{Changes: make([]syncManifestChange, 0, len(visible)), Cursor: formatCursor(cursor)}
	highest := cursor
	for _, e := range entries {
		if e.Seq > highest {
			highest = e.Seq
		}
	}
	for _, e := range visible {
		resp.Changes = append(resp.Changes, syncManifestChange{
			Kind:    e.Kind,
			ID:      manifestKey(e),
			Typ:     e.Typ,
			Deleted: e.Deleted,
		})
	}
	resp.Cursor = formatCursor(highest)
	writeV1JSON(w, http.StatusOK, resp)
}

// filterVisibleManifest drops every manifest entry the request principal may
// not read, preserving order. Reads are gated by entity type:
//
//   - An entity entry (Kind "e") gates on its own (Typ, IDA). Entity tombstones
//     carry Typ from the deletions table, so they gate the same way.
//   - A relation entry (Kind "r") has no type of its own; it gates on its source
//     (From = IDA) entity, mirroring handleV1EntityRelations. The source's type
//     is resolved from the store (empty if the source is gone — the same
//     fallback the relation write gate uses).
//
// Probes are batched per type via PermitsReadMany so the whole manifest costs
// one MatchingIDs roundtrip per distinct type, not one per row.
func (a *App) filterVisibleManifest(ctx context.Context, entries []synctypes.ManifestEntry) ([]synctypes.ManifestEntry, error) {
	gate := readGateFromContext(ctx)

	// Resolve the gating (type, id) for each entry, collecting ids per type.
	type gateKey struct{ typ, id string }
	keys := make([]gateKey, len(entries))
	idsByType := map[string][]string{}
	for i, e := range entries {
		typ := e.Typ
		id := e.IDA
		if e.Kind == "r" {
			// A relation gates on its source entity (IDA = From).
			if src, err := a.store.GetEntity(ctx, e.IDA); err == nil {
				typ = src.Type
			} else {
				typ = ""
			}
		}
		keys[i] = gateKey{typ: typ, id: id}
		idsByType[typ] = append(idsByType[typ], id)
	}

	// One batched permission probe per distinct type.
	permByType := make(map[string]map[string]bool, len(idsByType))
	for typ, ids := range idsByType {
		perm, err := gate.PermitsReadMany(ctx, typ, ids)
		if err != nil {
			return nil, err
		}
		permByType[typ] = perm
	}

	visible := make([]synctypes.ManifestEntry, 0, len(entries))
	for i, e := range entries {
		if permByType[keys[i].typ][keys[i].id] {
			visible = append(visible, e)
		}
	}
	return visible, nil
}

// handleSyncRecord dispatches /api/sync/<kind>/<id...> by method:
//
//	GET    -> fetch the record's full content
//	PUT    -> conditional push (If-Match: <hash>); 200 / 412 / 422
//	DELETE -> conditional delete (If-Match: <hash>); 200 / 412
//
// kind is "entities" or "relations". For an entity the id is the path tail; for
// a relation the tail is "<from>/<relType>/<to>".
func (a *App) handleSyncRecord(w http.ResponseWriter, r *http.Request) {
	kind, rest, ok := splitSyncPath(r.URL.Path)
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Unknown sync resource", "")
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.handleSyncGet(w, r, kind, rest)
	case http.MethodPut:
		a.handleSyncPut(w, r, kind, rest)
	case http.MethodDelete:
		a.handleSyncDelete(w, r, kind, rest)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, PUT, or DELETE", "")
	}
}

// --- path parsing + validation ---

// splitSyncPath parses /api/sync/<kind>/<rest> into (kind, rest). kind must be
// "entities" or "relations". rest is the remaining path (an entity id, or a
// relation's from/type/to segments).
func splitSyncPath(path string) (kind, rest string, ok bool) {
	tail := strings.TrimPrefix(path, "/api/sync/")
	if tail == path {
		return "", "", false
	}
	parts := strings.SplitN(tail, "/", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", false
	}
	if parts[0] != "entities" && parts[0] != "relations" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// validIDSegment allowlists an id/key path segment: it must be non-empty and
// contain no path-traversal or separator characters. This runs BEFORE the
// store so a crafted id can never escape the intended key space.
func validIDSegment(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, "/\\") || strings.Contains(s, "..") {
		return false
	}
	for _, c := range s {
		if c < 0x20 { // no control characters
			return false
		}
	}
	return true
}

func parseCursor(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0 // malformed cursor degrades to a full manifest, never an error
	}
	return n
}

func formatCursor(n int64) string { return strconv.FormatInt(n, 10) }

// manifestKey renders a ManifestEntry's key the way the wire id field expects —
// and, crucially, the SAME way the record path encodes it, so the client can
// use a manifest entry's id directly as the path tail. An entity is its id; a
// relation is "from/type/to" (slash-joined, matching parseRelationKey). Slashes
// cannot appear in a segment (validIDSegment rejects them), so the slash join is
// unambiguous — unlike a "--" delimiter, which a segment may legally contain.
func manifestKey(e synctypes.ManifestEntry) string {
	if e.Kind == "r" {
		return e.IDA + "/" + e.IDB + "/" + e.IDC
	}
	return e.IDA
}
