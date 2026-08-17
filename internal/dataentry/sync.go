package dataentry

import (
	"context"
	"net/http"
	"strconv"

	synctypes "github.com/Sourcehaven-BV/rela/internal/sync"
)

// --- Consumer-side interfaces (declared at the call site per CLAUDE.md) ---

// manifestProvider is the sync manifest source. Only pgstore implements it
// (sync is fs-client ↔ pg-server), so it is wired optionally: nil on the
// fs/memory builds, where the manifest endpoint returns 501.
type manifestProvider interface {
	ManifestSince(ctx context.Context, cursor int64) ([]synctypes.ManifestEntry, error)
}

// --- Wire DTOs ---

type syncManifestResponse struct {
	Changes []syncManifestChange `json:"changes"`
	Cursor  string               `json:"cursor"`
}

type syncManifestChange struct {
	Kind    string `json:"kind"` // "e" or "r"
	ID      string `json:"id"`   // entity id, or "from/type/to" for a relation
	Typ     string `json:"typ,omitempty"`
	Deleted bool   `json:"deleted"`
}

// handleSyncManifest: GET /api/sync/manifest?cursor=<token>. Returns the changes
// since the cursor and a new cursor (the highest seq seen). The cursor is a
// server-minted token the client stores and echoes back; today it is the seq
// watermark rendered as a decimal string (the client must treat it as opaque
// and not derive meaning from it — the encoding may change). A missing or
// malformed cursor is treated as 0 (full manifest), which is the safe degrade:
// the client re-bootstraps rather than silently skipping changes.
func (h *syncHandler) handleSyncManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET", "")
		return
	}
	mp := h.manifest
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
	visible, err := h.filterVisibleManifest(r.Context(), entries)
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
func (h *syncHandler) filterVisibleManifest(
	ctx context.Context, entries []synctypes.ManifestEntry,
) ([]synctypes.ManifestEntry, error) {
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
			if src, err := h.store.GetEntity(ctx, e.IDA); err == nil {
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

// --- cursor + key helpers ---

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
