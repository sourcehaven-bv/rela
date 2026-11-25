package dataentry

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// handleV1RelationHistory serves a relation's version history (postgres only):
//
//	GET  /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}            → timeline
//	GET  /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}/{version}  → one snapshot
//	POST /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}/{version}/restore
//
// The {fromType} segment is cosmetic and never an ACL input. Reads are gated
// on both endpoints (see relationHistoryReadable); a denial is the same 404 as
// a missing relation. Meta is redacted against the live source entity (see
// serveRelationHistoryVersion).
func handleV1RelationHistory(a *App, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_relation_history/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_relation_history/{fromType}/{from}/{relType}/{to}[/{version}[/restore]]", "")
		return
	}
	fromRef, refErr := entityPkg.ParseRef(parts[1])
	if refErr != nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	from, relType, to := fromRef.ID, parts[2], parts[3]

	if a.versions == nil {
		writeV1Error(w, r, http.StatusNotImplemented, "history_unsupported",
			"The active storage backend does not support relation version history", "")
		return
	}
	var reader store.RelationHistoryReader = a.versions

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
		restoreRelationHistoryVersion(a, w, r, reader, fromRef, relType, to, parts[4])
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

	if !authorizeRelationHistoryRead(a, w, r, fromRef, to) {
		return
	}

	if len(parts) >= 5 && parts[4] == "_lifetimes" {
		serveRelationLifetimes(w, r, reader, fromRef, relType, to)
		return
	}

	recordID, ok := parseRecordID(r)
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_record_id",
			"record_id must be a positive integer", "")
		return
	}
	q := store.RelationHistoryQuery{
		Key: entityPkg.RelationKey{From: from, FromFace: fromRef.Face, Type: relType, To: to}, RecordID: recordID,
	}

	if len(parts) >= 5 && parts[4] != "" {
		serveRelationHistoryVersion(a, w, r, reader, q, parts[4])
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
// relation's history, writing an indistinguishable-404 otherwise. The rule
// is [relationHistoryReadable]; read and restore both call this.
func authorizeRelationHistoryRead(
	a *App, w http.ResponseWriter, r *http.Request, from entityPkg.Ref, to string,
) bool {
	ok, err := relationHistoryReadable(r.Context(), a.visibleReader, from, to)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// relationHistoryReadable decides whether the ctx principal may read the
// history of the relation from → to. ok=false is the uniform not-found; an
// error is a gate failure. Every end that still exists must pass its live read
// check; a deleted tail face also needs the principal's grants to cover it,
// as in [resolveHistorySubject]. When any part is gone, the global
// acl.PermHistoryRead is required too. Each end's type is its stored type,
// never the URL. A failed stored-faces read is an error, never "deleted",
// because the deleted rule grants on a global permission.
func relationHistoryReadable(
	ctx context.Context, vr visibleReader, from entityPkg.Ref, to string,
) (bool, error) {
	fromType, fromFaces, err := loadStoredFaces(ctx, vr.store, from.ID)
	if err != nil {
		return false, err
	}
	toType, _, err := loadStoredFaces(ctx, vr.store, to)
	if err != nil {
		return false, err
	}

	partlyGone := false
	deletedTailFace := false
	var ok bool
	switch {
	case fromType == "":
		partlyGone = true
		ok = true
	case from.Face.IsImplicit():
		_, ok, err = vr.family(ctx, fromType, from.ID)
	case slices.Contains(fromFaces, from.Face):
		_, ok, err = vr.ref(ctx, fromType, from)
	default:
		partlyGone, deletedTailFace = true, true
		_, ok, err = vr.family(ctx, fromType, from.ID)
	}
	if err != nil || !ok {
		return false, err
	}

	if toType == "" {
		partlyGone = true
	} else if _, ok, err = vr.family(ctx, toType, to); err != nil || !ok {
		return false, err
	}

	if !partlyGone {
		return true, nil
	}
	if !readGateFromContext(ctx).HoldsPermission(ctx, acl.PermHistoryRead) {
		return false, nil
	}
	if deletedTailFace {
		if worldFromContext(ctx).blocksAllReads() {
			return false, nil
		}
		if !faceReadable(ctx, fromType, from.Face) {
			return false, nil
		}
	}
	return true, nil
}

// serveRelationLifetimes writes the list of a key's past lifetimes (newest-first),
// so a UI can offer a lifetime picker for a deleted-and-recreated relation.
func serveRelationLifetimes(
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	fromRef entityPkg.Ref, relType, to string,
) {
	lifetimes, err := reader.ListRelationLifetimes(r.Context(), entityPkg.RelationKey{
		From: fromRef.ID, FromFace: fromRef.Face, Type: relType, To: to,
	})
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
		"from": fromRef.String(), "type": relType, "to": to, "lifetimes": rows,
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
		"from": q.Key.From, "type": q.Key.Type, "to": q.Key.To, "versions": versions,
	})
}

// serveRelationHistoryVersion writes one relation version's full snapshot for
// the lifetime the query selects. Field redaction follows the current ACL
// against the live source entity: a live source redacts the frozen meta per
// field as the live relation GET does; a deleted source serves no meta to
// anyone, because its type is unrecoverable and the URL fromType must never
// key a grant. Timeline metadata (version, op, who, when) is served in both
// cases.
func serveRelationHistoryVersion(
	a *App, w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	q store.RelationHistoryQuery, versionStr string,
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

	meta := map[string]any{}
	vr := a.visibleReader
	if srcType := vr.storedType(ctx, snap.From); srcType != "" {
		srcRef := entityPkg.Ref{ID: snap.From, Face: q.Key.FromFace}
		src, live, gateErr := vr.ref(ctx, srcType, srcRef)
		if gateErr != nil {
			writeGateError(w, r, gateErr)
			return
		}
		if live {
			meta = a.affordances.visibleRelationMeta(ctx, src, snap.Type, cloneProps(snap.Properties))
		}
	}

	row := map[string]any{
		"from": snap.From, "type": snap.Type, "to": snap.To,
		"content": snap.Content, "meta": meta,
	}
	writeV1JSON(w, http.StatusOK, map[string]any{
		"from":       q.Key.From,
		"type":       q.Key.Type,
		"to":         q.Key.To,
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
// Reads are gated before the write, so a caller who cannot read the relation
// gets the same 404 as for a missing one.
func restoreRelationHistoryVersion(a *App,
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	fromRef entityPkg.Ref, relType, to, versionStr string,
) {
	from := fromRef.ID
	if !authorizeRelationHistoryRead(a, w, r, fromRef, to) {
		return
	}
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}

	ctx := r.Context()
	q := store.RelationHistoryQuery{Key: entityPkg.RelationKey{
		From: from, FromFace: fromRef.Face, Type: relType, To: to,
	}}
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
	key := entityPkg.RelationKey{From: from, FromFace: fromRef.Face, Type: relType, To: to}

	live, liveErr := a.store.GetRelation(ctx, key)
	if liveErr != nil {
		live = nil
	}
	if !restoreAffordanceOK(a, w, r, key, live, opts.Properties) {
		return
	}
	var writeErr error
	if live != nil {
		_, writeErr = a.entityManager.UpdateRelation(ctx, key, opts)
	} else {
		_, writeErr = a.entityManager.CreateRelation(ctx, key, opts)
	}
	if writeErr != nil {
		if writeForbiddenIfACLDenied(w, r, writeErr) || writeCardinalityIfExceeded(w, r, writeErr) {
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
		"relation":              map[string]any{"from": fromRef.String(), "type": relType, "to": to},
	})
}

// restoreAffordanceOK gates a restore as the PATCH that makes the same change
// is gated: a re-create needs the edge to be creatable, and every meta value
// the restore changes must be writable. Both are judged over the sources
// [affordanceService.relationSources] returns for the edge's source at its
// tail, so an identity edge from a faced source answers for every face. It
// writes the answer itself when the restore may not proceed.
//
// live is the edge as stored, or nil when the restore re-creates it.
func restoreAffordanceOK(
	a *App, w http.ResponseWriter, r *http.Request,
	key entityPkg.RelationKey, live *entityPkg.Relation, props map[string]any,
) bool {
	ctx := r.Context()
	sources, err := a.affordances.relationSources(ctx, nil,
		entityPkg.Ref{ID: key.From, Face: key.FromFace}, string(DirectionIncoming), key.Type)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	changed := props
	if live == nil {
		if src, denial := a.affordances.relationOpDenial(ctx, sources, key.Type, RelationOpCreate); denial != nil {
			a.denyAffordance(ctx, w, src, *denial)
			return false
		}
	} else {
		changed = make(map[string]any)
		for k, v := range props {
			if cur, ok := live.Properties[k]; !ok || !reflect.DeepEqual(cur, v) {
				changed[k] = v
			}
		}
	}
	if src, denial := a.affordances.relationMetaDenial(ctx, sources, key.Type, changed, nil); denial != nil {
		a.denyAffordance(ctx, w, src, *denial)
		return false
	}
	return true
}
