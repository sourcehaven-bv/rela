package dataentry

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// handleV1RelationHistory serves a relation's version history (postgres-backed
// only). A relation is addressed by its composite key. The leading `{fromType}`
// URL segment is COSMETIC — it is never an ACL trust input (see the note at the
// parts split, and authorizeRelationHistoryRead).
//
// Routes:
//
//	GET  /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}            → timeline
//	GET  /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}/{version}  → one snapshot
//	POST /api/v1/_relation_history/{fromType}/{from}/{relType}/{to}/{version}/restore
//
// Security (design-review RR-SDDYZO): relation-history read is gated on BOTH
// endpoints' read verdicts (FROM ∧ TO), each resolved against the endpoint's
// LIVE type. The "FROM entity owns the history" UI decision governs placement
// only — it must NOT become the authorization boundary, or the TO endpoint would
// be an existence/content oracle for a principal who can read FROM but not TO.
// When part of the relation is gone, every end that still exists keeps its
// check and the global acl.PermHistoryRead is required on top; a denial is the
// same 404 as a nonexistent relation (see relationHistoryReadable).
//
// Field redaction (TKT-B1F5Q1, IB-review #1): relation meta supports field-level
// `visible:` redaction, on the live relation GET and here in history. History
// redaction is governed by the CURRENT LIVE world evaluated against the LIVE
// source entity — a live source redacts per-field against today's policy; a
// deleted source serves NO meta to anyone (no reveal). See
// serveRelationHistoryVersion.
func handleV1RelationHistory(a *App, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_relation_history/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Need at least {fromType}/{from}/{relType}/{to}.
	if len(parts) < 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_relation_history/{fromType}/{from}/{relType}/{to}[/{version}[/restore]]", "")
		return
	}
	// parts[0] is the {fromType} URL segment. It is COSMETIC only — never an ACL
	// trust input (the gate + redaction resolve the real type from the globally-
	// unique id; see authorizeRelationHistoryRead / serveRelationHistoryVersion).
	//
	// parts[1] is an ADDRESS, not a bare id: on a faced type `selfHref` hands out
	// `ID@face` and the SPA passes it here verbatim, so it must be parsed rather
	// than handed to the store (TKT-JAROC3). The face selects which TAIL's history
	// this is; the bare id is what every ACL row gate keys on.
	fromRef, refErr := entityPkg.ParseRef(parts[1])
	if refErr != nil {
		// An address the grammar rejects names no row. Same not-found a missing
		// relation gives — a distinct 400 would only say which strings are worth
		// probing.
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

	// GET .../_lifetimes enumerates every past lifetime of a reused key. Same read
	// gate as the timeline (just authorized above). The body carries only lifetime
	// metadata (counts/timestamps/final-op), never relation content — but it DOES
	// reveal that older deleted lifetimes exist, which is the feature's whole point
	// and is what the gate authorizes.
	if len(parts) >= 5 && parts[4] == "_lifetimes" {
		serveRelationLifetimes(w, r, reader, fromRef, relType, to)
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
		// Denied on any end → indistinguishable 404. Never reveal which.
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// relationHistoryReadable decides whether the ctx principal may read the
// history of the relation from → to. ok=false is the uniform not-found; an
// error is a gate failure.
//
// The rule is "both ends, as far as they still exist" (RR-SDDYZO). Every end
// that still exists must pass its live read check:
//
//   - A live tail is read as design 8.2 says: a named face as that face, a
//     bare tail as the entity (some readable face of the id).
//   - A named tail face that was deleted while the entity lives on must pass
//     the entity's row gate on its remaining faces, and the principal's read
//     grants must cover the deleted face. This mirrors the deleted-face rule
//     of entity history ([resolveHistorySubject]).
//   - A live head is an entity-level check.
//
// When any part of the relation is gone (a deleted tail face, or an end
// with no stored row at all), the history is deleted-relation history and
// additionally requires the global acl.PermHistoryRead. An end with no
// stored row has no verdict left to evaluate and no trustworthy type, so it
// adds no check of its own: unlike entity history, which takes the type from
// the URL, the `{fromType}` segment here is never an ACL input.
//
// Each end's type is its STORED type, never the URL: rela ids are globally
// unique, so the stored header yields the real type (IB-review #1). A failed
// stored-faces read is an error, never "deleted", because the deleted rule
// grants on a global permission.
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
		// A named face in a denied world is refused, as the live face is.
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
		"from": q.Key.From, "type": q.Key.Type, "to": q.Key.To, "versions": versions,
	})
}

// serveRelationHistoryVersion writes one relation version's full snapshot for the
// lifetime the query selects.
//
// Field redaction is governed by the CURRENT, LIVE ACL world evaluated against
// the LIVE source entity (TKT-B1F5Q1, IB-review #1). Relation history is a lens
// onto data whose access is decided by today's policy and today's graph — not by
// anything reconstructed from the moment of capture. Two cases:
//
//   - The source entity is LIVE → redact the frozen meta per-field against the
//     current relation `visible:` policy resolved for the live source (exactly
//     the live relation GET does). Lose a role today and you lose historical
//     visibility; gain one and you gain it — reader-side access is always live.
//
//   - The source entity is DELETED → serve NO meta, to everyone. The thing that
//     grants access to a relation's properties is the live relation; once its
//     source is gone there is nothing to evaluate current ACL against, and its
//     type is unrecoverable (the version row stores the id, not the type — so we
//     must never trust the caller-supplied URL fromType to key a grant, or a
//     principal could spoof a type their own role has a favorable grant for).
//     There is deliberately NO history:read-redacted reveal for a deleted
//     relation: gone is gone, uniformly. (This is the intentional divergence from
//     ENTITY history, which keeps a reconstruct-and-reveal model — a relation's
//     access is a property of the live relation, so it cannot outlive it.)
//
// Timeline/attribution metadata (version, op, who, when) still serves in both
// cases — that is the history feature working; only the ACL-governed property
// values follow the live-world rule.
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

	// Live source → redact against today's policy; deleted source → no meta.
	meta := map[string]any{}
	// The source is the row at the tail this history is for. A miss (gone, or
	// not readable) serves no meta, which fails closed.
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
func restoreRelationHistoryVersion(a *App,
	w http.ResponseWriter, r *http.Request, reader store.RelationHistoryReader,
	fromRef entityPkg.Ref, relType, to, versionStr string,
) {
	from := fromRef.ID
	// Restore is a write; gate reads first so a caller who can't even read the
	// history can't probe it via restore.
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
	// Restore reads from the newest lifetime (RecordID 0); the HTTP restore route
	// does not expose an older-lifetime selector (the CLI does via --lifetime).
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
	// The restore writes back to the TAIL it read from. Without it a faced
	// edge's history would restore onto the DEFAULT tail, a different
	// relation, leaving the edge the caller addressed untouched and minting
	// a second one beside it (TKT-JAROC3).
	key := entityPkg.RelationKey{From: from, FromFace: fromRef.Face, Type: relType, To: to}

	// If the relation currently exists, update it; else re-create. Both go
	// through the entitymanager, which authorizes, validates endpoints, and
	// audits. A missing endpoint surfaces as ErrEntityNotFound → 409.
	//
	// Liveness is probed on the addressed tail for the same reason: a probe
	// of the default tail would find a live faced edge absent and take the
	// create branch.
	_, liveErr := a.store.GetRelation(ctx, key)
	var writeErr error
	if liveErr == nil {
		_, writeErr = a.entityManager.UpdateRelation(ctx, key, opts)
	} else {
		_, writeErr = a.entityManager.CreateRelation(ctx, key, opts)
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
		"relation":              map[string]any{"from": fromRef.String(), "type": relType, "to": to},
	})
}
