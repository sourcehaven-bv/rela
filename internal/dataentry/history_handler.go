package dataentry

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// revealIsPrivileged reports whether taking the reveal arm actually constitutes
// a privileged disclosure worth auditing, which is only true under a configured
// policy.
//
// Without this the audit row is worse than useless. Under NopACL and
// ReadOnlyACL no middleware attaches a read gate, so readGateFromContext hands
// back nopReadGate, whose HoldsPermission returns true for EVERY permission
// (readgate.go:135, the RR-CWWJGW shape). Every history read would therefore
// take the reveal arm — but with no policy configured nothing is redacted, so
// those reads reveal nothing. Recording them would bury the real reveals under
// noise in every unconfigured deployment, and would train an operator who later
// configures a policy to ignore exactly the row this exists to surface.
//
// A closed switch on the ACL IMPLEMENTATION, matching permitsGatedUIElement:
// asking the read gate here is precisely the fail-open mistake being avoided,
// since the gate is the thing that cannot answer. Value and pointer forms are
// both matched because these types' methods have value receivers. An
// implementation nobody taught this about audits (the default arm) — the
// conservative direction for a log, where a spurious row is recoverable and a
// missing one is not.
func revealIsPrivileged(aclImpl acl.ACL) bool {
	switch aclImpl.(type) {
	case nil:
		// Wired without an ACL: same "no policy" case as NopACL.
		return false
	case acl.NopACL, *acl.NopACL, acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return false
	default:
		return true
	}
}

// recordHistoryReveal emits the audit row for a history read that overrode
// redaction via acl.PermHistoryReadRedacted (TKT-LVSPSB / issue #1238).
//
// entityType MUST come from the stored snapshot rather than the caller-supplied
// URL segment: the recorded type is forensic evidence, and taking it from the
// request would let a caller write a type of their choosing into the audit log.
//
// No revealed values and no revealed field names are recorded -- see
// audit.OpHistoryReveal for why the field list is itself sensitive.
//
// The reveal is not blocked on the audit write succeeding; sink errors are the
// sink's concern, exactly as for every other op.
//
// A free function taking the sink, not a method on App: App is at its
// plimsoll method cap, and this needs exactly one field of it. Passing the
// dependency also makes the function directly testable without an App.
func recordHistoryReveal(ctx context.Context, sink audit.Audit, entityType, entityID string, version int) {
	sink.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpHistoryReveal,
		Subject:     &audit.Subject{Kind: "entity", Type: entityType, ID: entityID},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     "history_reveal=true version=" + strconv.Itoa(version),
	})
}

// handleV1History serves an entity face's version history (database
// backends only).
//
// Routes:
//
//	GET  /api/v1/_history/{type}/{addr}                   → the version timeline (metadata)
//	GET  /api/v1/_history/{type}/{addr}/{version}         → one version's full snapshot
//	POST /api/v1/_history/{type}/{addr}/{version}/restore → restore that version
//
// {addr} is `ID@face` or a bare `ID`. A lineage is keyed by (id, face), so
// the address resolves to ONE face and every read and write below is scoped
// to it (BUG-4SYAA6). See [resolveHistorySubject] for how the face is chosen
// and who may read it.
//
// Security (design-review findings):
//   - A LIVE face's history is gated by the SAME resolver read as the entity
//     GET, so a hidden, mistyped or nonexistent address is an
//     indistinguishable 404 (RR-KDXGYK / RR-NGMI).
//   - A DELETED face has no per-entity verdict to evaluate (its conferring
//     relations are gone), so its history requires the global
//     acl.PermHistoryRead permission. A NON-holder gets the SAME 404 as a
//     nonexistent id, never a 403 that would confirm it ever existed.
//   - Every snapshot is rendered through the serializer's forWire so
//     field-level (`visible:`) redaction strips hidden properties exactly as
//     on a live GET (RR-YDMJV7).
//   - A restore reads the same way before it writes, then writes as an
//     update (or, for a deleted face, a create) of `type@face`.
func handleV1History(a *App, w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_history/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_history/{type}/{id}[/{version}[/restore]]", "")
		return
	}
	typeName := parts[0]
	ref, refErr := entityPkg.ParseRef(parts[1])
	if refErr != nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	if a.versions == nil {
		// A backend with no version-history capability — fsstore (which gets
		// history from git instead) or memstore. This is a capability gap,
		// not an ACL decision, so it's safe to say so plainly.
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
			// Unreachable: restore refuses `?world=`, and only a world makes
			// a subject absent. Answered as absent, never as a write.
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return
		}
		writeV1JSON(w, http.StatusOK, map[string]any{
			"id": ref.ID, "versions": []map[string]any{}, "world_face_absent": true,
		})
		return
	}
	// Every read below names subject.ref, so it is scoped to the one face
	// the subject resolved to: the store's history is per face.
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
// deleted marks a face with no live row, whose history was opened on the
// global history permission. Such a timeline is served only when it is
// non-empty and every version is of the URL type: an empty one would tell a
// permission holder "no such lineage" apart from the 404 of a hidden live
// face, and a mismatched one would open another type's lineage under this
// type's face grant.
func serveHistoryTimeline(
	w http.ResponseWriter, r *http.Request, reader store.HistoryReader,
	typeName string, subjectRef entityPkg.Ref, deleted bool,
) {
	entityID, face := subjectRef.ID, subjectRef.Face
	metas, err := reader.ListVersions(r.Context(), subjectRef)
	if err != nil {
		// Scrub backend detail from the wire (RR-372L): a store error must not
		// echo table/column names.
		writeGateError(w, r, err)
		return
	}
	if deleted && !lineageOfType(metas, typeName) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
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
	// The response NAMES the face it belongs to, and HOW that face was
	// chosen. A record that does not name its subject invites the reader to
	// assume the obvious one, which is precisely how the default face's
	// history passed for a published page's. Empty face means the default
	// face, matching the face's own zero value.
	//
	// `via` matters more here than on a read surface. A world may answer with
	// a STAND-IN face (`otherwise: default`, rule 3), which is a good answer
	// for a reader — English beats a blank page for someone who asked for
	// Dutch — and a misleading one for a timeline, because a history labeled
	// only by face looks like the one the caller asked for. Labeling it the
	// way the entity GET does (see worldProvenance) is what keeps the two
	// surfaces from disagreeing about the same resolution.
	rule, position := resolutionRuleAt(worldFromContext(ctx).scope, typeName, face)
	body := map[string]any{
		"id": entityID, "versions": versions, "face": face.String(), "via": rule,
	}
	if position != nil {
		body["chain_position"] = *position
	}
	writeV1JSON(w, http.StatusOK, body)
}

// lineageOfType reports whether metas is non-empty and every version is of
// typeName.
func lineageOfType(metas []store.VersionMeta, typeName string) bool {
	if len(metas) == 0 {
		return false
	}
	for _, m := range metas {
		if m.Type != typeName {
			return false
		}
	}
	return true
}

// serveHistoryVersion writes one version's full snapshot, redacted through the
// serializer so hidden (`visible:`-denied) properties never reach the client.
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
	// The snapshot's type must match the URL type — otherwise a deleted entity
	// of type A could be read via /_history/B/<A-id> under B's read verdict
	// (the cross-type leak). The face must match too, as restore checks: the
	// row gate ran for subjectRef.Face, so a snapshot of another face would
	// be served under the wrong grant. Mismatch → indistinguishable 404.
	if snap.Type != typeName || snap.Face != subjectRef.Face {
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
	snapEntity.Face = snap.Face
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
		// Record the privileged disclosure, not the read (TKT-LVSPSB / issue
		// #1238). Only this arm, and only under a configured policy: an
		// ordinary redacted read discloses nothing the permission governs, and
		// under no policy this arm is reached by every reader with nothing
		// redacted to reveal. Both would bury the real reveals this record
		// exists to surface. See audit.OpHistoryReveal and revealIsPrivileged.
		if revealIsPrivileged(a.acl) {
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
	// Same gate as the timeline, over the single meta this snapshot carries.
	sources := gateOriginSources(ctx, readGateFromContext(ctx), []store.VersionMeta{snap.VersionMeta})
	if o := originWire(snap.Origin, sources[0]); o != nil {
		payload["origin"] = o
	}
	writeV1JSON(w, http.StatusOK, payload)
}
