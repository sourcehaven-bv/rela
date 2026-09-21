package dataentry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/transform"
)

// exportSegment is the reserved final path segment that turns a document URL
// into an export. It is safe to overload the /_documents/ path this way for the
// same reason the entity router can (see the `_export` case in
// handleV1EntityRoute): an entity ID can never collide with it, because
// entity.ValidateID rejects a leading underscore outright.
//
// That rule is a WELL-FORMEDNESS rule, not a security control — its own godoc
// says so and warns against leaning on it. Routing is a legitimate use of it
// (an ambiguous route has no structural alternative, and the failure mode is a
// shadowed route, not a privilege escalation), but it IS a premise, so it is
// stated here and pinned by a guard test.
//
// The other half of the path is closed at config load: validateDocuments
// rejects a document named "_export". This is an alias of the constant that
// rule uses rather than a second literal, so the reservation and the route
// cannot drift apart — renaming one is a compile error in the other.
const exportSegment = dataentryconfig.ReservedExportSegment

// resolvedDocument is what the gate chain produces: a render config for a
// document the caller has been authorized to render, plus the entry entity ID
// ("" for a standalone document).
//
// It deliberately carries nothing else. Everything the HTML path additionally
// needs — the return_to rewrite target, the refresh flag, the disk cache — is
// specific to producing HTML and stays in that handler. See the note on
// resolveAnchoredDocument about GetCached in particular.
type resolvedDocument struct {
	cfg     documentRenderConfig
	entryID string
}

// resolveStandaloneDocument runs the full gate chain for a standalone document
// and returns its render config. On any denial it has already written the
// response and returns ok=false.
//
// This exists so the render and export paths cannot drift. The gates are
// ORDERED and the order is load-bearing; two copies of an ordered security
// chain is how one copy quietly loses a gate.
func resolveStandaloneDocument(
	a *App, w http.ResponseWriter, r *http.Request, docName string,
) (resolvedDocument, bool) {
	if docName == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_documents/{docName} or /_documents/{docName}/{entityId}", "")
		return resolvedDocument{}, false
	}

	// Validated even though a standalone render writes no cache file today:
	// docName reaches the script engine as a config key, and the guard costs
	// nothing. Keeping it means a future cache keyed on docName cannot
	// reintroduce a traversal.
	if !isSafePathSegment(docName) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path segment contains forbidden characters", "")
		return resolvedDocument{}, false
	}

	docCfg, ok := a.State().Cfg.Documents[docName]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "document_not_found", "Document config not found", "")
		return resolvedDocument{}, false
	}

	// The mirror of the standalone rejection in handleV1Documents: an
	// entity-anchored document needs an entry id this shape cannot supply.
	// Rejecting beats rendering it against a guessed or empty entity.
	if !docCfg.IsStandalone() {
		writeV1Error(w, r, http.StatusBadRequest, "document_kind_mismatch",
			documentKindMismatchAnchored(docName, docCfg), "")
		return resolvedDocument{}, false
	}

	// Gate BEFORE rendering — see gateDocumentPermission. Both gates apply:
	// the permission check, and (for an elevated document) the closed switch
	// on the ACL implementation that the read gate alone cannot provide.
	if !gateDocumentPermission(w, r, docName, docCfg) {
		return resolvedDocument{}, false
	}
	if !gateElevatedDocument(w, r, a.acl, docName, docCfg) {
		return resolvedDocument{}, false
	}

	return resolvedDocument{cfg: a.toDocumentRenderConfig(docName, &docCfg)}, true
}

// resolveAnchoredDocument runs the full gate chain for an entity-anchored
// document and returns its render config plus the entry entity ID. On any
// denial it has already written the response and returns ok=false.
//
// Gate ORDER is the whole point of this function, and two orderings are
// load-bearing:
//
//   - The ACL read gate runs BEFORE the store read. A hidden entity and a
//     nonexistent one must be indistinguishable — in the response body AND in
//     timing. Reading the store first (as this code did before TKT-K7J6FL) made
//     a missing id answer "entity %q not found" while a denied id answered the
//     uniform entityNotFoundTitle, so a principal who could not read the type
//     could still enumerate which ids existed. That is the RR-NGMI invariant
//     handleV1GetEntity protects by gating first, for the same reason.
//   - The type-mismatch check runs BELOW the read gate, so a denied principal
//     gets the uniform 404 rather than a 400 that reveals the entity's type
//     (pinned by TestAnchoredDocument_GateOrderingNoTypeOracle).
//
// What this function deliberately does NOT do is touch the document cache.
// documentService.GetCached keys on the entry id and content hash with neither
// the ConfigID nor the principal in the key, so it is only safe where it is
// used today: reads skipped for script: documents, writes only for
// principal-independent command: renders. A per-principal caller reading that
// cache would reintroduce RR-2QSGLU.
func resolveAnchoredDocument(
	a *App, w http.ResponseWriter, r *http.Request, docName, entityID string,
) (resolvedDocument, bool) {
	// Both segments flow into the on-disk document cache filename
	// (document.go GetCached/doRender). Reject anything that could escape the
	// cache directory before any filesystem work happens.
	//
	// The entity segment is an ADDRESS — `ID` or `ID@face` — so it is
	// validated as a state ref rather than a plain segment (BUG-VFHUWO).
	// isSafeStateRefSegment applies isSafePathSegment to the bare id and
	// entity.ParseFace to the suffix, so neither half can carry a separator;
	// `@` itself is inert in a filename, and the faced address keys a cache
	// entry distinct from the bare one, which is correct — they render
	// different content.
	//
	// A malformed address is a 400 here rather than the uniform 404 the entity
	// routes give one, and that difference is deliberate: this segment also
	// names a CACHE FILE, so "this path is unusable" is the honest answer and
	// the caller learns nothing about which entities exist — the id never
	// reaches the store. It is also what keeps a reserved segment in the entity
	// position (`/_documents/sales/_EXPORT`) a 400: `parseEntityRef` rejects a
	// leading underscore, which is exactly the case this route must not serve.
	ref, refOK := parseEntityRef(entityID)
	if !isSafePathSegment(docName) || !isSafeStateRefSegment(entityID) || !refOK {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path segment contains forbidden characters", "")
		return resolvedDocument{}, false
	}

	docCfg, ok := a.State().Cfg.Documents[docName]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "document_not_found", "Document config not found", "")
		return resolvedDocument{}, false
	}

	// A standalone document has no entry entity, so an entity-anchored
	// request for one is a category error, not a document to render against
	// the supplied id. Reject rather than silently ignoring the id.
	if docCfg.IsStandalone() {
		writeV1Error(w, r, http.StatusBadRequest, "document_kind_mismatch",
			documentKindMismatchStandalone(docName), "")
		return resolvedDocument{}, false
	}

	// ACL gate (TKT-C0R07J), FIRST — see the ordering note in the doc comment.
	// Document rendering serves entity-derived content and may run a Lua script
	// that reads related entities, so a denied caller must never reach the
	// renderer, and must not learn whether the id exists.
	// The BARE id: the row gate is face-blind by design.
	if !a.gateReadOrNotFound(w, r, docCfg.EntityType, ref.ID) {
		return resolvedDocument{}, false
	}

	// A doc-level `permission:` applies IN ADDITION to the per-entity gate
	// above — it narrows, never widens (a holder still needs to pass the
	// entity read gate).
	if !gateDocumentPermission(w, r, docName, docCfg) {
		return resolvedDocument{}, false
	}

	// An entity-anchored document may also be elevated (TKT-Y3JVFK), in which
	// case BOTH this and the per-entity gate above must pass. The entity gate
	// governs the entry entity; elevation governs what the script may read
	// BEYOND it, so neither subsumes the other.
	if !gateElevatedDocument(w, r, a.acl, docName, docCfg) {
		return resolvedDocument{}, false
	}

	// Enforce the doc's entity_type: a release-notes script authored for
	// releases must not run against a ticket. The frontend already filters the
	// docs shown for an entity, but an HTTP caller can hit
	// /_documents/<doc>/<wrong-type-id> directly.
	//
	// The read is safe here because the gate above already authorized this
	// principal for this id; a miss gets the SAME uniform 404 a denial gets,
	// so the two remain indistinguishable (entityNotFoundTitle's godoc
	// requires exactly this).
	ent, entErr := a.store.GetEntityState(r.Context(), ref.ID, ref.Face)
	if entErr != nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return resolvedDocument{}, false
	}
	if ent.Type != docCfg.EntityType {
		writeV1Error(w, r, http.StatusBadRequest, "entity_type_mismatch",
			documentTypeMismatch(docName, docCfg.EntityType, entityID, ent.Type), "")
		return resolvedDocument{}, false
	}

	return resolvedDocument{cfg: a.toDocumentRenderConfig(docName, &docCfg), entryID: entityID}, true
}

// handleV1ExportDocument serves the document export routes:
//
//	GET /api/v1/_documents/{docName}/_export?transform=<name>
//	GET /api/v1/_documents/{docName}/{entityId}/_export?transform=<name>
//
// Export is a READ affordance strictly downstream of the same gate chain the
// HTML render uses — resolve* above IS that chain, shared rather than retyped.
// A request may only choose a registered transform NAME; never a command, flag,
// path, or renderer. The response is hardened like an attachment download
// because a transform emits attacker-influenceable bytes.
//
// entityID is "" for the standalone shape.
func handleV1ExportDocument(a *App, w http.ResponseWriter, r *http.Request, docName, entityID string) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Transform resolution first: it is pure caller input, and rejecting a bad
	// one costs nothing. It reveals only which formats are configured, which is
	// already public via GET /_transforms.
	name, reg, ok := a.export.resolveTransform(w, r)
	if !ok {
		return
	}

	var resolved resolvedDocument
	if entityID == "" {
		resolved, ok = resolveStandaloneDocument(a, w, r, docName)
	} else {
		resolved, ok = resolveAnchoredDocument(a, w, r, docName, entityID)
	}
	if !ok {
		return // the resolver already wrote the response
	}

	// A command: renderer is refused on BOTH document shapes. For a standalone
	// document there is no entry entity to hand it as {in} at all; for an
	// anchored one it would work, so this half is a policy choice — supporting
	// export on one document kind but not the other is a worse contract than
	// supporting neither, and a command: render's output is the one thing the
	// entry-hash disk cache is built for. Script-only keeps the two kinds
	// symmetric. The render layer carries the same assertion as a backstop
	// (RenderStandaloneMarkdown); this one exists to give the operator a clear
	// 400 instead of a generic render failure.
	if len(resolved.cfg.Command) > 0 {
		writeV1Error(w, r, http.StatusBadRequest, "export_unsupported_renderer",
			"This document cannot be exported",
			"export requires a script: renderer; command: documents are not exportable")
		return
	}

	renderer := transform.RendererFunc(func(ctx context.Context) ([]byte, error) {
		md, err := a.documents.RenderDocumentMarkdown(ctx, resolved.entryID, resolved.cfg)
		if err != nil {
			return nil, err
		}
		return []byte(md), nil
	})

	a.export.convertAndWrite(w, r, reg, name, renderer, exportBaseName(docName, entityID),
		"document", docName)
}

// exportBaseName is the download filename stem: the document name for a
// standalone export, "<doc>-<id>" when anchored to an entity. Both components
// have already passed isSafePathSegment, and safeAttachmentFilename sanitizes
// the result again downstream.
func exportBaseName(docName, entityID string) string {
	if entityID == "" {
		return docName
	}
	return docName + "-" + entityID
}

// documentKindMismatchAnchored is the message for an entity-anchored document
// requested at the standalone URL shape.
func documentKindMismatchAnchored(docName string, docCfg dataentryconfig.DocumentConfig) string {
	return fmt.Sprintf("document %q is for entity_type %q; request it at /_documents/%s/{entityId}",
		docName, docCfg.EntityType, docName)
}

// documentKindMismatchStandalone is the message for a standalone document
// requested at the entity-anchored URL shape.
func documentKindMismatchStandalone(docName string) string {
	return fmt.Sprintf("document %q has no entity_type; request it at /_documents/%s without an entity id",
		docName, docName)
}

// documentTypeMismatch is the message for an entity whose type is not the one
// the document declares.
func documentTypeMismatch(docName, wantType, entityID, gotType string) string {
	return fmt.Sprintf("document %q is for entity_type %q, but %q is a %q",
		docName, wantType, entityID, gotType)
}
