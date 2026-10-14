package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/access"
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
// The decision per ACL mode is [access.Mode.PermitsElevatedRead]. It keys on
// the ACL implementation, not the read gate, and denies under NopACL as well:
// unlike commands, an elevated document has no pre-ACL behavior to preserve,
// so granting would leave the boundary inert while it looks configured.
func authorizeElevatedDocument(
	ctx context.Context, aclImpl acl.ACL, docCfg dataentryconfig.DocumentConfig,
) bool {
	if !docCfg.AllowACLBypass.Enabled() {
		return true // not elevated: gateDocumentPermission is the whole story.
	}
	return access.ModeOf(aclImpl).PermitsElevatedRead(ctx, readGateFromContext(ctx), docCfg.Permission)
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
	resolved, ok := resolveStandaloneDocument(a, w, r, docName)
	if !ok {
		return // the resolver already wrote the response
	}

	returnPath := isSafeReturnPath(r.URL.Query().Get("return_to"))

	html, err := a.documents.RenderStandalone(r.Context(), resolved.cfg)
	if err != nil {
		var se *lua.ScriptError
		if errors.As(err, &se) {
			correlationID := newCorrelationID()
			slog.Warn("standalone document render failed",
				"document", docName, "correlation", correlationID, "error", err)
			writeV1ScriptError(w, se, a.allowFullScriptDetail(r), correlationID)
			return
		}
		writeInternalError(w, r, "render_failed", "Document rendering failed", err)
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
