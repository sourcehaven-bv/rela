package dataentry

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// recordHistoryReveal emits the audit row for a history read that overrode
// redaction via acl.PermHistoryReadRedacted.
//
// entityType MUST come from the stored snapshot, not the URL, so a caller
// cannot write a type of their choosing into the audit log. No revealed
// values or field names are recorded (see audit.OpHistoryReveal). Sink errors
// do not block the reveal.
func recordHistoryReveal(ctx context.Context, sink audit.Audit, entityType, entityID string, version int) {
	sink.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpHistoryReveal,
		Subject:     &audit.Subject{Kind: "entity", Type: entityType, ID: entityID},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		AssertedBy:  audit.AssertedByFrom(ctx),
		Summary:     "history_reveal=true version=" + strconv.Itoa(version),
	})
}

// handleV1History serves an entity face's version history (database backends
// only): the timeline, one version's snapshot, or a restore of that version.
// {addr} resolves to ONE face (see [resolveHistorySubject]).
//
// A LIVE face is gated by the same resolver read as the entity GET, so a
// hidden or nonexistent address is an indistinguishable 404. A DELETED face
// requires acl.PermHistoryRead, and a non-holder gets the same 404. Every
// snapshot is redacted like a live GET. A restore reads the same way first.
func handleV1History(a *App, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_history/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_history/{type}/{id}[/{version}[/restore]]", "")
		return
	}
	typeName := parts[0]
	ref, refErr := parseHistoryAddress(parts[1])
	if refErr != nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	if a.versions == nil {
		writeV1Error(w, r, http.StatusNotImplemented, "history_unsupported",
			"The active storage backend does not support version history", "")
		return
	}

	restore := len(parts) == 4 && parts[3] == "restore"
	if !historyMethodAllowed(w, r, restore) {
		return
	}

	subject, ok := historySubjectOr404(w, r, a.visibleReader, typeName, ref)
	if !ok {
		return
	}
	if subject.worldAbsent {
		if restore {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return
		}
		writeV1JSON(w, http.StatusOK, map[string]any{
			"id": ref.ID, "versions": []map[string]any{}, "world_face_absent": true,
		})
		return
	}
	switch {
	case restore:
		restoreHistoryVersion(a, w, r, a.versions, typeName, subject, parts[2])
	case len(parts) >= 3 && parts[2] != "":
		serveHistoryVersion(a, w, r, a.versions, typeName, subject.ref, parts[2])
	default:
		serveHistoryTimeline(w, r, a.versions, typeName, subject.ref, subject.live == nil)
	}
}

// historyMethodAllowed accepts POST for a restore and GET otherwise, and
// writes the refusal (or the OPTIONS answer) itself.
func historyMethodAllowed(w http.ResponseWriter, r *http.Request, restore bool) bool {
	want, allow := http.MethodGet, "GET, OPTIONS"
	if restore {
		want, allow = http.MethodPost, "POST, OPTIONS"
	}
	if r.Method == want {
		return true
	}
	w.Header().Set("Allow", allow)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	return false
}

// serveHistoryTimeline writes the version metadata list (oldest first).
//
// Versions of another type or face in the lineage are dropped: the row gate
// ran for this type and face only. Copy-origin source ids are gated against
// the reader's own verdict. A deleted face's timeline is served only when
// non-empty, so it cannot be told apart from a hidden live face's 404.
func serveHistoryTimeline(
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader,
	typeName string, subjectRef entityPkg.Ref, deleted bool,
) {
	entityID, face := subjectRef.ID, subjectRef.Face
	metas, err := reader.ListVersions(r.Context(), subjectRef)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	metas = versionsAt(metas, typeName, face)
	if deleted && len(metas) == 0 {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	ctx := r.Context()
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
			if m.PrevFace != m.Face {
				row["prev_face"] = m.PrevFace.String()
			}
		}
		if m.TriggeredBy != "" {
			row["triggered_by"] = m.TriggeredBy
		}
		if o := originWire(m.Origin, sources[i]); o != nil {
			row["origin"] = o
		}
		versions = append(versions, row)
	}
	rule, position := resolutionRuleAt(worldFromContext(ctx).scope, typeName, face)
	body := map[string]any{
		"id": entityID, "versions": versions, "face": face.String(), "via": rule,
	}
	if position != nil {
		body["chain_position"] = *position
	}
	writeV1JSON(w, http.StatusOK, body)
}

// versionsAt returns the versions of metas that are of typeName at face.
func versionsAt(metas []store.VersionMeta, typeName string, face entityPkg.Face) []store.VersionMeta {
	out := make([]store.VersionMeta, 0, len(metas))
	for _, m := range metas {
		if m.Type == typeName && m.Face == face {
			out = append(out, m)
		}
	}
	return out
}

// serveHistoryVersion writes one version's full snapshot, redacted through the
// serializer so `visible:`-denied properties never reach the client. A
// snapshot of another type or face than the URL names is an
// indistinguishable 404. Historical redaction fails closed; a holder of
// acl.PermHistoryReadRedacted sees all frozen fields, and that reveal is
// audited.
func serveHistoryVersion(a *App,
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader,
	typeName string, subjectRef entityPkg.Ref, versionStr string,
) {
	version, convErr := strconv.Atoi(versionStr)
	if convErr != nil || version < 1 {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_version",
			"Version must be a positive integer", "")
		return
	}

	entityID := subjectRef.ID
	snap, err := reader.GetVersion(r.Context(), subjectRef, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	if snap.Type != typeName || snap.Face != subjectRef.Face {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	ctx := r.Context()
	snapEntity := entityPkg.New(entityID, snap.Type)
	snapEntity.Face = snap.Face
	snapEntity.Content = snap.Content
	snapEntity.Properties = cloneProps(snap.Properties)
	meta := a.Meta()
	plural := typeName
	if def, ok := meta.GetEntityDef(snap.Type); ok {
		plural = def.GetPlural(snap.Type)
	}
	var wire v1.Entity
	if readGateFromContext(ctx).HoldsPermission(ctx, acl.PermHistoryReadRedacted) {
		wire = a.serializer.forWireHistoricalReveal(ctx, snapEntity, meta, plural)
		if access.ModeOf(a.acl).RevealIsPrivileged() {
			recordHistoryReveal(ctx, a.auditSink, snap.Type, entityID, snap.Version)
		}
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
	sources := gateOriginSources(ctx, readGateFromContext(ctx), []store.VersionMeta{snap.VersionMeta})
	if o := originWire(snap.Origin, sources[0]); o != nil {
		payload["origin"] = o
	}
	writeV1JSON(w, http.StatusOK, payload)
}

// parseHistoryAddress parses the id segment of a per-face history route
// (`_history`, `_changes`). The result names the face whose lineage is read
// and goes to [historySubjectOr404], which resolves a bare id in the
// request's world.
func parseHistoryAddress(segment string) (entityPkg.Ref, error) {
	return entityPkg.ParseRef(segment)
}
