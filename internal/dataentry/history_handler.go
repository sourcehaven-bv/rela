package dataentry

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
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
//     entity (getEntity + type check + gateReadOrNotFound), so a hidden-or-nonexistent id
//     returns an indistinguishable 404 (RR-KDXGYK / RR-NGMI).
//   - A DELETED entity has no per-entity verdict to evaluate (its conferring
//     relations are gone), so its history requires the global acl.PermHistoryRead
//     permission. A NON-holder gets the SAME 404 as a nonexistent id — never a
//     403 that would confirm the deleted entity ever existed.
//   - Every snapshot is rendered through the serializer's forWire so field-level
//     (`visible:`) redaction strips hidden properties exactly as on a live GET
//     (RR-YDMJV7) — a raw snapshot would bypass the serializer-layer redaction.
func handleV1History(a *App, w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/_history/{type}/{id}[/{version}[/restore]]
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_history/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_history/{type}/{id}[/{version}[/restore]]", "")
		return
	}
	typeName, entityID := parts[0], parts[1]

	if a.versions == nil {
		// Non-postgres backend: no version history capability. This is a
		// capability gap, not an ACL decision, so it's safe to say so plainly.
		writeV1Error(w, r, http.StatusNotImplemented, "history_unsupported",
			"The active storage backend does not support version history", "")
		return
	}
	var reader store.HistoryReader = a.versions

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
		restoreHistoryVersion(a, w, r, reader, typeName, entityID, parts[2])
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
	if !authorizeHistoryRead(a, w, r, typeName, entityID) {
		return
	}

	// Narrow the reader to the FACE the request's world resolves (BUG-2).
	// Versioning is per-face, so a world-bound page asking for "the history"
	// means the history of the face on screen — serving the default face's
	// instead is the wrong record presented as the right one.
	//
	// This runs AFTER authorizeHistoryRead, so the face is only ever resolved
	// for a caller already cleared to read the entity.
	face, present, ferr := historyFace(r.Context(), a.store, entityID)
	if ferr != nil {
		writeGateError(w, r, ferr)
		return
	}
	if !present {
		// The world resolves no face for this entity, so there is no history
		// in this world. An EMPTY timeline, not a 404: the entity exists and
		// this caller may read it (the gate above said so), and a 404 here
		// would contradict the entity view, which answers the same question
		// with `_world_absent` and the default face.
		writeV1JSON(w, http.StatusOK, map[string]any{
			"id": entityID, "versions": []map[string]any{}, "world_face_absent": true,
		})
		return
	}
	// The row gate in authorizeHistoryRead is face-blind; the face being
	// served — the default one, or the world's resolution — must pass the
	// face half too, or a `type@published` principal reads the draft's
	// timeline and snapshots here while the entity GET 404s.
	if !faceReadable(r.Context(), typeName, face) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	scoped, capable := faceHistoryReader(reader, face)
	if !capable {
		// The backend has entity history but not the FACE-scoped capability, so
		// it cannot answer this question. Refuse rather than serving the
		// default face's history under a world — a wrong record is worse than a
		// named refusal (the same posture the 501 above takes).
		writeV1Error(w, r, http.StatusNotImplemented, "history_face_unsupported",
			"The active storage backend cannot serve per-face version history",
			"omit ?world= to read the default face's history")
		return
	}

	if len(parts) >= 3 && parts[2] != "" {
		serveHistoryVersion(a, w, r, scoped, typeName, entityID, parts[2])
		return
	}
	serveHistoryTimeline(w, r, scoped, entityID, face)
}

// authorizeHistoryRead returns true if the caller may read this entity's
// history, writing the appropriate (indistinguishable-404) response otherwise.
//
// The URL {type} is attacker-controlled and the store keys history by ID ONLY,
// so the type MUST be checked against the entity's real type — otherwise a
// principal denied type A but allowed type B could request /_history/B/<A-id>
// and read A's history under B's (permissive) read verdict (a confused-deputy
// cross-type leak). The live GET handler makes the same check (entity.Type !=
// typeName ⇒ 404); the version-read/restore paths additionally verify the
// SNAPSHOT's type matches (see verifySnapshotType), so a deleted entity of a
// mismatched type is a 404 too.
func authorizeHistoryRead(a *App, w http.ResponseWriter, r *http.Request, typeName, entityID string) bool {
	ctx := r.Context()
	gate := readGateFromContext(ctx)

	// Live entity: gate exactly as a GET would (PermitsRead), so a hidden or
	// nonexistent id is an indistinguishable 404. A type mismatch is ALSO a 404
	// (indistinguishable), so the URL type can't be used to borrow another
	// type's read verdict.
	if live, found := a.reader.getEntity(ctx, entityID); found {
		if live.Type != typeName {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return false
		}
		return a.gateReadOrNotFound(w, r, typeName, entityID)
	}

	// Not live: either genuinely absent, or deleted-with-surviving-history.
	// Reading a deleted entity's history requires the global permission; a
	// non-holder gets the SAME 404 as a nonexistent id (no existence oracle).
	// The deleted entity's type is verified against the URL when a snapshot is
	// read (verifySnapshotType); the timeline endpoint exposes only metadata.
	if !gate.HoldsPermission(ctx, acl.PermHistoryRead) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// serveHistoryTimeline writes the version metadata list (oldest first).
func serveHistoryTimeline(
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader,
	entityID string, face entityPkg.Face,
) {
	metas, err := reader.ListVersions(r.Context(), entityID)
	if err != nil {
		// Scrub backend detail from the wire (RR-372L): a store error must not
		// echo table/column names.
		writeGateError(w, r, err)
		return
	}
	ctx := r.Context()
	// The source ids named by copy origins are ROWS, so they are gated
	// against this reader's own verdict before any of them reaches the wire.
	sources := gateOriginSources(ctx, readGateFromContext(ctx), metas)
	versions := make([]map[string]any, 0, len(metas))
	for i, m := range metas {
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
		// Omitted entirely for a direct edit — the absence is the signal that
		// a human typed this version, and `principal` above says who.
		if o := originWire(m.Origin, sources[i]); o != nil {
			row["origin"] = o
		}
		versions = append(versions, row)
	}
	// The response NAMES the face it belongs to. A record that does not name
	// its subject invites the reader to assume the obvious one, which is
	// precisely how the default face's history passed for a published page's.
	// Empty means the default face, matching the face's own zero value.
	writeV1JSON(w, http.StatusOK, map[string]any{
		"id": entityID, "versions": versions, "face": face.String(),
	})
}

// serveHistoryVersion writes one version's full snapshot, redacted through the
// serializer so hidden (`visible:`-denied) properties never reach the client.
func serveHistoryVersion(a *App,
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
	// The snapshot's type must match the URL type — otherwise a deleted entity
	// of type A could be read via /_history/B/<A-id> under B's read verdict
	// (the cross-type leak, see authorizeHistoryRead). Mismatch → indistinguishable 404.
	if snap.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	// Reconstruct the entity as-of this version and route it through the
	// serializer so field-level redaction (stripHiddenProperties) applies — a
	// raw snapshot would leak `visible:`-denied properties (RR-YDMJV7).
	//
	// Historical redaction FAILS CLOSED (TKT-73C6B2). Two reader tiers:
	//
	//   - Ordinary reader: the ctx is marked historical-subject, so a conditional
	//     `visible:` grant whose subject-world inputs (has_relation /
	//     count_relations) can't be affirmed for this possibly-deleted/drifted
	//     entity evaluates false and HIDES the field. The live store no longer
	//     holds the entity's as-of-version edges, so trusting it would let such a
	//     grant flip OPEN and leak a field hidden at write time (the old
	//     RR-TPATBK under-redaction). Reader-side inputs stay live (per-reader
	//     redaction is intended).
	//
	//   - Holder of acl.PermHistoryReadRedacted (audit super-user): bypass the
	//     strip entirely via forWireHistoricalReveal — sees ALL frozen fields
	//     (OVERRIDE semantics, sibling of PermHistoryRead). Skipping only the
	//     historical marker would NOT be enough: the ordinary live strip would
	//     still run and hide fields the live policy redacts, which is not the
	//     all-or-nothing reveal this permission grants.
	ctx := r.Context()
	snapEntity := entityPkg.New(entityID, snap.Type)
	snapEntity.Content = snap.Content
	snapEntity.Properties = cloneProps(snap.Properties) // N1: don't alias the snapshot map
	meta := a.Meta()
	plural := typeName
	if def, ok := meta.GetEntityDef(snap.Type); ok {
		plural = def.GetPlural(snap.Type)
	}
	var wire v1.Entity
	if readGateFromContext(ctx).HoldsPermission(ctx, acl.PermHistoryReadRedacted) {
		wire = a.serializer.forWireHistoricalReveal(ctx, snapEntity, meta, plural)
	} else {
		wire = a.serializer.forWire(affordances.WithHistoricalSubject(ctx), snapEntity, nil, meta, plural)
	}

	payload := map[string]any{
		"id":         entityID,
		"version":    snap.Version,
		"op":         snap.Op,
		"created_at": snap.CreatedAt,
		"principal":  map[string]string{"user": snap.PrincipalUser, "tool": snap.PrincipalTool},
		"entity":     wire,
	}
	// Same gate as the timeline, over the single meta this snapshot carries.
	sources := gateOriginSources(ctx, readGateFromContext(ctx), []store.VersionMeta{snap.VersionMeta})
	if o := originWire(snap.Origin, sources[0]); o != nil {
		payload["origin"] = o
	}
	writeV1JSON(w, http.StatusOK, payload)
}
