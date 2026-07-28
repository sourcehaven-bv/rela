package dataentry

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
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
// Field redaction (TKT-B1F5Q1, was RR-BZNL0S): relation meta now supports
// field-level `visible:` redaction, both on a live relation GET and here in
// history. Relation history therefore exposes exactly what a live relation GET
// exposes — no more — and fails CLOSED for a possibly-deleted/drifted relation
// (deny-by-default, same rule + acl.PermHistoryReadRedacted reveal as the entity
// path). See serveRelationHistoryVersion.
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

	if a.versions == nil {
		writeV1Error(w, r, http.StatusNotImplemented, "history_unsupported",
			"The active storage backend does not support relation version history", "")
		return
	}
	var reader store.RelationHistoryReader = a.versions

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

	// GET .../_lifetimes enumerates every past lifetime of a reused key. Same read
	// gate as the timeline (just authorized above). The body carries only lifetime
	// metadata (counts/timestamps/final-op), never relation content — but it DOES
	// reveal that older deleted lifetimes exist, which is the feature's whole point
	// and is what the gate authorizes.
	if len(parts) >= 5 && parts[4] == "_lifetimes" {
		serveRelationLifetimes(w, r, reader, from, relType, to)
		return
	}

	// Optional ?record_id=<n> selects a specific past lifetime (0/absent = newest).
	// The store validates membership in the key's lifetimes, so a client-supplied
	// record_id cannot escape the composite-key auth boundary. A non-empty but
	// unparseable value is a 400 (never silently coerced to newest — that would
	// serve a different lifetime than asked for, with no signal).
	recordID, ok := parseRecordID(r)
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_record_id",
			"record_id must be a positive integer", "")
		return
	}
	q := store.RelationHistoryQuery{From: from, Type: relType, To: to, RecordID: recordID}

	if len(parts) >= 5 && parts[4] != "" {
		serveRelationHistoryVersion(a, w, r, reader, fromType, q, parts[4])
		return
	}
	serveRelationHistoryTimeline(w, r, reader, q)
}

// parseRecordID reads the optional ?record_id= lifetime selector. Absent → (0,
// true) = newest. A non-empty value must parse to a positive integer; otherwise
// (0, false) so the caller returns 400 rather than silently serving the newest.
func parseRecordID(r *http.Request) (int64, bool) {
	raw := r.URL.Query().Get("record_id")
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
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

// serveRelationLifetimes writes the list of a key's past lifetimes (newest-first),
// so a UI can offer a lifetime picker for a deleted-and-recreated relation.
func serveRelationLifetimes(
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	from, relType, to string,
) {
	lifetimes, err := reader.ListRelationLifetimes(r.Context(), from, relType, to)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	rows := make([]map[string]any, 0, len(lifetimes))
	for _, lt := range lifetimes {
		rows = append(rows, map[string]any{
			"lifetime":      lt.Lifetime,
			"record_id":     lt.RecordID,
			"version_count": lt.VersionCount,
			"first_seen":    lt.FirstSeen,
			"last_seen":     lt.LastSeen,
			"live":          lt.Live,
			"final_op":      lt.FinalOp,
		})
	}
	writeV1JSON(w, http.StatusOK, map[string]any{
		"from": from, "type": relType, "to": to, "lifetimes": rows,
	})
}

// serveRelationHistoryTimeline writes the version metadata list for the lifetime
// the query selects.
func serveRelationHistoryTimeline(
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	q store.RelationHistoryQuery,
) {
	metas, err := reader.ListRelationVersions(r.Context(), q)
	if errors.Is(err, store.ErrNotFound) {
		// A record_id that is not a lifetime of this key: same indistinguishable
		// 404 as a nonexistent relation (never confirm the id belongs elsewhere).
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
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
		"from": q.From, "type": q.Type, "to": q.To, "versions": versions,
	})
}

// serveRelationHistoryVersion writes one relation version's full snapshot for the
// lifetime the query selects.
//
// Field redaction fails closed (TKT-B1F5Q1, inheriting TKT-73C6B2): the frozen
// relation meta is redacted against TODAY's relation `visible:` policy through a
// historical-subject ctx, so any grant whose subject-world inputs cannot be
// affirmed for a possibly-deleted/drifted relation HIDES the field rather than
// leaking it. fromType (from the URL) keys the source's relation grant block; the
// snapshot itself carries no live entity, so a minimal source entity is
// synthesized — under the historical marker the resolver neuters subject-world
// lookups anyway. A holder of acl.PermHistoryReadRedacted bypasses the strip
// entirely (OVERRIDE reveal), exactly as serveHistoryVersion does for entities.
func serveRelationHistoryVersion(
	a *App, w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	fromType string, q store.RelationHistoryQuery, versionStr string,
) {
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}
	ctx := r.Context()
	snap, err := reader.GetRelationVersion(ctx, q, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	meta := cloneProps(snap.Properties) // don't alias the snapshot map
	if !readGateFromContext(ctx).HoldsPermission(ctx, acl.PermHistoryReadRedacted) {
		// Fail closed: resolve relation `visible:` under the historical marker. The
		// grant block is keyed by the source entity type; synthesize a minimal
		// source (the marker neuters subject-world lookups, so no live entity is
		// needed).
		src := entityPkg.New(snap.From, fromType)
		meta = a.affordances.visibleRelationMeta(
			affordances.WithHistoricalSubject(ctx), src, snap.Type, meta)
	}

	row := map[string]any{
		"from": snap.From, "type": snap.Type, "to": snap.To,
		"content": snap.Content, "meta": meta,
	}
	writeV1JSON(w, http.StatusOK, map[string]any{
		"from":       q.From,
		"type":       q.Type,
		"to":         q.To,
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
	// Restore reads from the newest lifetime (RecordID 0); the HTTP restore route
	// does not expose an older-lifetime selector (the CLI does via --lifetime).
	q := store.RelationHistoryQuery{From: from, Type: relType, To: to}
	snap, err := reader.GetRelationVersion(ctx, q, version)
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
