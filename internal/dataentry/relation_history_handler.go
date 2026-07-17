package dataentry

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// handleV1RelationHistory serves a relation's version history (postgres-backed
// only). A relation is addressed by its composite key plus the FROM entity's
// type (so the read gate can evaluate a per-type verdict on the from endpoint).
//
// Routes:
//
//	GET  /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}            → timeline
//	GET  /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}/{version}  → one snapshot
//	POST /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}/{version}/restore
//
// Security (design-review RR-SDDYZO): relation-history read is gated on BOTH
// endpoints' read verdicts (FROM ∧ TO). The "FROM entity owns the history" UI
// decision governs placement only — it must NOT become the authorization
// boundary, or the TO endpoint would be an existence/content oracle for a
// principal who can read FROM but not TO. A deleted relation (endpoints gone)
// requires the global acl.PermHistoryRead; a non-holder gets the same 404 as a
// nonexistent relation (no existence oracle).
//
// Field redaction (RR-BZNL0S): relations have NO field-level redaction anywhere
// in the live system today (unlike entities). Relation history therefore exposes
// exactly what a live relation GET exposes — no more. This is pinned by a test;
// a relation field-redaction path is a separate follow-up.
func handleV1RelationHistory(a *App, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_relation_history/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Need at least {fromType}/{from}/{relType}/{to}.
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_relation_history/{fromType}/{from}/{relType}/{to}[/{version}[/restore]]", "")
		return
	}
	fromType, from, relType, to := parts[0], parts[1], parts[2], parts[3]

	reader, ok := a.store.(store.RelationHistoryReader)
	if !ok {
		writeV1Error(w, r, http.StatusNotImplemented, "history_unsupported",
			"The active storage backend does not support relation version history", "")
		return
	}

	// POST .../{version}/restore is the one write on this route.
	if len(parts) == 6 && parts[5] == "restore" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
			return
		}
		restoreRelationHistoryVersion(a, w, r, reader, fromType, from, relType, to, parts[4])
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

	if !authorizeRelationHistoryRead(a, w, r, fromType, from, to) {
		return
	}

	if len(parts) >= 5 && parts[4] != "" {
		serveRelationHistoryVersion(w, r, reader, from, relType, to, parts[4])
		return
	}
	serveRelationHistoryTimeline(w, r, reader, from, relType, to)
}

// authorizeRelationHistoryRead returns true if the caller may read this
// relation's history, writing an indistinguishable-404 otherwise.
//
// Dual-endpoint gating (RR-SDDYZO): BOTH endpoints must be readable. If both are
// live, each must pass the per-type read verdict (the FROM type comes from the
// URL; the TO type from its live row). If either endpoint is not live, the
// relation's endpoints are (at least partly) gone — treat it as deleted-relation
// history and require the global PermHistoryRead, else the same 404 as a
// nonexistent relation.
func authorizeRelationHistoryRead(a *App, w http.ResponseWriter, r *http.Request, fromType, from, to string) bool {
	ctx := r.Context()
	gate := readGateFromContext(ctx)

	fromEntity, fromLive := a.reader.getEntity(ctx, from)
	toEntity, toLive := a.reader.getEntity(ctx, to)

	if fromLive && toLive {
		// From type must match the URL type (no borrowing another type's verdict).
		if fromEntity.Type != fromType {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return false
		}
		fromOK, err := gate.PermitsRead(ctx, fromType, from)
		if err != nil {
			writeGateError(w, r, err)
			return false
		}
		toOK, err := gate.PermitsRead(ctx, toEntity.Type, to)
		if err != nil {
			writeGateError(w, r, err)
			return false
		}
		if !fromOK || !toOK {
			// Denied on EITHER endpoint → indistinguishable 404. Never reveal which.
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return false
		}
		return true
	}

	// One or both endpoints gone: deleted-relation history. Global permission.
	if !gate.HoldsPermission(ctx, acl.PermHistoryRead) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// serveRelationHistoryTimeline writes the relation's version metadata list.
func serveRelationHistoryTimeline(
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	from, relType, to string,
) {
	metas, err := reader.ListRelationVersions(r.Context(), from, relType, to)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	versions := make([]map[string]any, 0, len(metas))
	for _, m := range metas {
		row := map[string]any{
			"version":    m.Version,
			"op":         m.Op,
			"from":       m.From,
			"type":       m.Type,
			"to":         m.To,
			"created_at": m.CreatedAt,
			"principal":  map[string]string{"user": m.PrincipalUser, "tool": m.PrincipalTool},
		}
		if m.PrevFrom != "" || m.PrevTo != "" {
			row["prev_from"] = m.PrevFrom
			row["prev_to"] = m.PrevTo
		}
		if m.TriggeredBy != "" {
			row["triggered_by"] = m.TriggeredBy
		}
		versions = append(versions, row)
	}
	writeV1JSON(w, http.StatusOK, map[string]any{
		"from": from, "type": relType, "to": to, "versions": versions,
	})
}

// serveRelationHistoryVersion writes one relation version's full snapshot.
func serveRelationHistoryVersion(
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	from, relType, to, versionStr string,
) {
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}
	snap, err := reader.GetRelationVersion(r.Context(), from, relType, to, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	row := map[string]any{
		"from": snap.From, "type": snap.Type, "to": snap.To,
		"content": snap.Content, "meta": snap.Properties,
	}
	writeV1JSON(w, http.StatusOK, map[string]any{
		"from":       from,
		"type":       relType,
		"to":         to,
		"version":    snap.Version,
		"op":         snap.Op,
		"created_at": snap.CreatedAt,
		"principal":  map[string]string{"user": snap.PrincipalUser, "tool": snap.PrincipalTool},
		"relation":   row,
	})
}

// restoreRelationHistoryVersion restores a relation's content + properties to a
// past version, via the entitymanager (RR-CCITK3) so the endpoint-existence
// check, type validation, and audit all run. If either endpoint entity no longer
// exists, the create maps ErrEntityNotFound → 409 dangling-edge (not 500).
func restoreRelationHistoryVersion(a *App,
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	fromType, from, relType, to, versionStr string,
) {
	// Restore is a write; gate reads first so a caller who can't even read the
	// history can't probe it via restore.
	if !authorizeRelationHistoryRead(a, w, r, fromType, from, to) {
		return
	}
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}

	ctx := r.Context()
	snap, err := reader.GetRelationVersion(ctx, from, relType, to, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	content := snap.Content
	opts := entityPkg.RelationOptions{
		Properties: cloneProps(snap.Properties),
		Content:    &content,
	}

	// If the relation currently exists, update it; else re-create. Both go
	// through the entitymanager, which authorizes, validates endpoints, and
	// audits. A missing endpoint surfaces as ErrEntityNotFound → 409.
	_, liveErr := a.store.GetRelation(ctx, from, relType, to)
	var writeErr error
	if liveErr == nil {
		_, writeErr = a.entityManager.UpdateRelation(ctx, from, relType, to, opts)
	} else {
		_, writeErr = a.entityManager.CreateRelation(ctx, from, relType, to, opts)
	}
	if writeErr != nil {
		if writeForbiddenIfACLDenied(w, writeErr) {
			return
		}
		if errors.Is(writeErr, entitymanager.ErrEntityNotFound) {
			writeV1Error(w, r, http.StatusConflict, "dangling_endpoint",
				"Cannot restore relation: an endpoint entity no longer exists", writeErr.Error())
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			"Relation restore failed validation", writeErr.Error())
		return
	}

	writeV1JSON(w, http.StatusOK, map[string]any{
		"restored_from_version": snap.Version,
		"relation":              map[string]any{"from": from, "type": relType, "to": to},
	})
}
