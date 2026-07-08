package dataentry

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// handleV1History serves an entity's version history (postgres-backed only).
//
// Routes:
//
//	GET /api/v1/_history/{type}/{id}           → the version timeline (metadata)
//	GET /api/v1/_history/{type}/{id}/{version} → one version's full snapshot
//
// Security (design-review findings):
//   - A LIVE entity's history is gated by the SAME read verdict as reading the
//     entity (getVisible / gateReadOrNotFound), so a hidden-or-nonexistent id
//     returns an indistinguishable 404 (RR-KDXGYK / RR-NGMI).
//   - A DELETED entity has no per-entity verdict to evaluate (its conferring
//     relations are gone), so its history requires the global acl.PermHistoryRead
//     permission. A NON-holder gets the SAME 404 as a nonexistent id — never a
//     403 that would confirm the deleted entity ever existed.
//   - Every snapshot is rendered through the serializer's forWire so field-level
//     (`visible:`) redaction strips hidden properties exactly as on a live GET
//     (RR-YDMJV7) — a raw snapshot would bypass the serializer-layer redaction.
func (a *App) handleV1History(w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/_history/{type}/{id}[/{version}[/restore]]
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_history/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_history/{type}/{id}[/{version}[/restore]]", "")
		return
	}
	typeName, entityID := parts[0], parts[1]

	reader, ok := a.store.(store.HistoryReader)
	if !ok {
		// Non-postgres backend: no version history capability. This is a
		// capability gap, not an ACL decision, so it's safe to say so plainly.
		writeV1Error(w, r, http.StatusNotImplemented, "history_unsupported",
			"The active storage backend does not support version history", "")
		return
	}

	// POST .../{version}/restore is the one write on this route.
	if len(parts) == 4 && parts[3] == "restore" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
			return
		}
		a.restoreHistoryVersion(w, r, reader, typeName, entityID, parts[2])
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Authorize reads: live entity → same read gate as a GET; deleted entity →
	// PermHistoryRead, else an indistinguishable 404.
	if !a.authorizeHistoryRead(w, r, typeName, entityID) {
		return
	}

	if len(parts) >= 3 && parts[2] != "" {
		a.serveHistoryVersion(w, r, reader, typeName, entityID, parts[2])
		return
	}
	a.serveHistoryTimeline(w, r, reader, entityID)
}

// authorizeHistoryRead returns true if the caller may read this entity's
// history, writing the appropriate (indistinguishable-404) response otherwise.
func (a *App) authorizeHistoryRead(w http.ResponseWriter, r *http.Request, typeName, entityID string) bool {
	ctx := r.Context()
	gate := readGateFromContext(ctx)

	// Live entity: gate exactly as a GET would (PermitsRead), so a hidden or
	// nonexistent id is an indistinguishable 404.
	_, found := a.reader.getEntity(ctx, entityID)
	if found {
		return a.gateReadOrNotFound(w, r, typeName, entityID)
	}

	// Not live: either genuinely absent, or deleted-with-surviving-history.
	// Reading a deleted entity's history requires the global permission; a
	// non-holder gets the SAME 404 as a nonexistent id (no existence oracle).
	if !gate.HoldsPermission(ctx, acl.PermHistoryRead) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// serveHistoryTimeline writes the version metadata list (oldest first).
func (a *App) serveHistoryTimeline(
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader, entityID string,
) {
	metas, err := reader.ListVersions(r.Context(), entityID)
	if err != nil {
		// Scrub backend detail from the wire (RR-372L): a store error must not
		// echo table/column names.
		writeGateError(w, r, err)
		return
	}
	versions := make([]map[string]any, 0, len(metas))
	for _, m := range metas {
		row := map[string]any{
			"version":    m.Version,
			"op":         m.Op,
			"type":       m.Type,
			"created_at": m.CreatedAt,
			"principal":  map[string]string{"user": m.PrincipalUser, "tool": m.PrincipalTool},
		}
		if m.PrevID != "" {
			row["prev_id"] = m.PrevID
		}
		if m.TriggeredBy != "" {
			row["triggered_by"] = m.TriggeredBy
		}
		versions = append(versions, row)
	}
	writeV1JSON(w, http.StatusOK, map[string]any{"id": entityID, "versions": versions})
}

// serveHistoryVersion writes one version's full snapshot, redacted through the
// serializer so hidden (`visible:`-denied) properties never reach the client.
func (a *App) serveHistoryVersion(
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader,
	typeName, entityID, versionStr string,
) {
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}

	snap, err := reader.GetVersion(r.Context(), entityID, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	// Reconstruct the entity as-of this version and route it through the
	// serializer so field-level redaction (stripHiddenProperties) applies — a
	// raw snapshot would leak `visible:`-denied properties (RR-YDMJV7).
	snapEntity := entityPkg.New(entityID, snap.Type)
	snapEntity.Content = snap.Content
	snapEntity.Properties = snap.Properties
	meta := a.Meta()
	plural := typeName
	if def, ok := meta.GetEntityDef(snap.Type); ok {
		plural = def.GetPlural(snap.Type)
	}
	wire := a.serializer.forWire(r.Context(), snapEntity, nil, meta, plural)

	writeV1JSON(w, http.StatusOK, map[string]any{
		"id":         entityID,
		"version":    snap.Version,
		"op":         snap.Op,
		"created_at": snap.CreatedAt,
		"principal":  map[string]string{"user": snap.PrincipalUser, "tool": snap.PrincipalTool},
		"entity":     wire,
	})
}
