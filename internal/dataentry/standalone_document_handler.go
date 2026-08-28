package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// gateDocumentPermission enforces a document's optional `permission:`.
// Returns true when the request may proceed; on deny it has already written
// the response.
//
// A document without a permission is ungated — that is the default, and it is
// deliberate. The confidentiality boundary for document content is the
// ACL-gated reader the render's Lua uses (lua.ReadDeps.VisibleReader), not
// this check: a principal who cannot read the underlying entities renders an
// empty or partial document either way. `permission:` exists for documents
// whose COMPOSITION is sensitive even though the parts are readable.
//
// The deny is a plain 403 NAMING the document, not a disguised 404. Which
// documents exist is not a secret — they are keys in `data-entry.yaml`, an
// operator-authored file in the repo (see "The configuration is not a secret;
// the data is" in the root CLAUDE.md). Concealing a config key would buy
// nothing and cost the operator a debuggable error. Contrast the entity-id
// path, where a uniform 404 IS required: whether an entity exists is a genuine
// secret.
//
// It fires before any renderer runs. Content would be safe regardless, but an
// unauthorized caller must not be able to trigger an expensive Lua aggregation.
//
// A plain function rather than an *App method: it needs only the request's
// read gate and the document's own config, and App sits on its plimsoll load
// line (see the directive on the type).
func gateDocumentPermission(
	w http.ResponseWriter, r *http.Request, docName string, docCfg dataentryconfig.DocumentConfig,
) bool {
	if docCfg.Permission == "" {
		return true
	}
	if readGateFromContext(r.Context()).HoldsPermission(r.Context(), docCfg.Permission) {
		return true
	}
	writeV1Error(w, r, http.StatusForbidden, "permission_required",
		fmt.Sprintf("document %q requires the %q permission", docName, docCfg.Permission), "")
	return false
}

// authorizeElevatedDocument is the boundary for a document that declares
// allow_acl_bypass (TKT-Y3JVFK). It is SEPARATE from gateDocumentPermission,
// and both must pass, because the two answer different questions.
//
// gateDocumentPermission consults the request's read gate. That is correct for
// an ordinary document, whose content is bounded by the ACL-gated reader
// regardless — but readGateFromContext hands back nopReadGate under BOTH
// acl.NopACL and acl.ReadOnlyACL, and its HoldsPermission returns TRUE
// (readgate.go). A predicate written against the read gate alone therefore
// FAILS OPEN, which is live bug RR-CWWJGW. For an elevated render the gate is
// the only thing between a principal and everything the script reads, so it
// keys on the ACL IMPLEMENTATION instead — the shape authorizeCommand uses and
// documents.
//
// Policy, mirroring authorizeCommand except where noted:
//
//   - nil ACL → deny. An authorization guard must fail closed on a wiring bug.
//   - [acl.ReadOnlyACL] → deny, value and pointer forms both (matching only
//     the value form was the one-'&' bypass authorizeCommand's doc describes).
//   - [acl.NopACL] → DENY. This DIVERGES from authorizeCommand, whose NopACL
//     arm grants in order to preserve pre-ACL behavior. An elevated document
//     has no pre-ACL behavior to preserve — the feature is new — so granting
//     would only create a deployment in which the boundary is inert while
//     appearing configured. Note validateDocuments already requires a
//     permission on every elevated document, so reaching here under NopACL
//     means a policy-less deployment declared elevation; refusing to serve is
//     the honest answer.
//   - [*acl.Declarative] → nil-check, then the permission must be held.
//   - anything else → DENY.
//
// The switch is closed by construction: an ACL implementation nobody taught
// this function about cannot silently grant an ungated raw read.
func authorizeElevatedDocument(
	ctx context.Context, aclImpl acl.ACL, docCfg dataentryconfig.DocumentConfig,
) bool {
	if !docCfg.AllowACLBypass.Enabled() {
		return true // not elevated: gateDocumentPermission is the whole story.
	}
	if aclImpl == nil {
		return false
	}

	switch a := aclImpl.(type) {
	case acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return false

	case acl.NopACL, *acl.NopACL:
		return false

	case *acl.Declarative:
		if a == nil {
			return false // misconfigured policy must not fail open
		}
		if docCfg.Permission == "" {
			// Unreachable via a validated config (validateDocuments rejects
			// this), but a guard that depends on validation having run is a
			// guard waiting to be bypassed by a new construction path.
			return false
		}
		return readGateFromContext(ctx).HoldsPermission(ctx, docCfg.Permission)

	default:
		return false
	}
}

// gateElevatedDocument applies authorizeElevatedDocument and writes the deny
// response. Split from the predicate so the decision stays testable without a
// ResponseWriter.
//
// The deny names the document and the permission for the same reason
// gateDocumentPermission does: which documents exist is config, not a secret.
func gateElevatedDocument(
	w http.ResponseWriter, r *http.Request, aclImpl acl.ACL,
	docName string, docCfg dataentryconfig.DocumentConfig,
) bool {
	if authorizeElevatedDocument(r.Context(), aclImpl, docCfg) {
		return true
	}
	writeV1Error(w, r, http.StatusForbidden, "permission_required",
		fmt.Sprintf("document %q reads with elevated privileges and requires the %q permission "+
			"under a configured acl.yaml", docName, docCfg.Permission), "")
	return false
}

// handleV1StandaloneDocument handles GET /api/v1/_documents/{docName} — the
// entity-less path shape, serving a document declared without an
// `entity_type:` (TKT-M1AX6P). Its content is company-wide (e.g. a sales
// report aggregated across many types) rather than about one entity, so there
// is no entry id in the URL and rela.document.entry_id is nil.
//
// docName is the raw first path segment; it may be empty when the caller hit
// /api/v1/_documents/ with no name at all.
//
// A plain function taking *App rather than a method on it: App sits on its
// plimsoll load line (see the directive on the type), and this is a leaf
// handler with no reason to widen App's surface.
func handleV1StandaloneDocument(a *App, w http.ResponseWriter, r *http.Request, docName string) {
	if docName == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_documents/{docName} or /_documents/{docName}/{entityId}", "")
		return
	}

	// Validated even though a standalone render writes no cache file today:
	// docName reaches the script engine as a config key, and the guard costs
	// nothing. Keeping it means a future cache keyed on docName cannot
	// reintroduce a traversal.
	if !isSafePathSegment(docName) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path segment contains forbidden characters", "")
		return
	}

	docCfg, ok := a.State().Cfg.Documents[docName]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "document_not_found", "Document config not found", "")
		return
	}

	// The mirror of the standalone rejection in handleV1Documents: an
	// entity-anchored document needs an entry id this shape cannot supply.
	// Rejecting beats rendering it against a guessed or empty entity.
	if !docCfg.IsStandalone() {
		writeV1Error(w, r, http.StatusBadRequest, "document_kind_mismatch",
			fmt.Sprintf("document %q is for entity_type %q; request it at /_documents/%s/{entityId}",
				docName, docCfg.EntityType, docName), "")
		return
	}

	// Gate BEFORE rendering — see gateDocumentPermission. Both gates apply:
	// the permission check, and (for an elevated document) the closed switch
	// on the ACL implementation that the read gate alone cannot provide.
	if !gateDocumentPermission(w, r, docName, docCfg) {
		return
	}
	if !gateElevatedDocument(w, r, a.acl, docName, docCfg) {
		return
	}

	returnPath := isSafeReturnPath(r.URL.Query().Get("return_to"))

	html, err := a.documents.RenderStandalone(r.Context(), a.toDocumentRenderConfig(docName, &docCfg))
	if err != nil {
		var se *lua.ScriptError
		if errors.As(err, &se) {
			correlationID := newCorrelationID()
			slog.Warn("standalone document render failed",
				"document", docName, "correlation", correlationID, "error", err)
			writeV1ScriptError(w, se, a.allowFullScriptDetail(r), correlationID)
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "render_failed", "Document rendering failed", err.Error())
		return
	}

	// EntityIDs is empty by construction: it drives the SPA's SSE live-reload
	// subscription, and a standalone document has no entry entity whose
	// changes would define staleness. Reloading is the refresh button until
	// TKT-E1FO1 (rela.document.depends_on) gives scripts a way to declare
	// their real dependencies. Cached is likewise always false — a standalone
	// render has no entry hash to key a disk cache on, so `?refresh=true` is
	// accepted and ignored rather than forgotten.
	writeV1JSON(w, http.StatusOK, v1.DocumentResponse{
		HTML:      RewriteDocumentLinks(html, returnPath, nil),
		Cached:    false,
		EntityIDs: []string{},
	})
}
