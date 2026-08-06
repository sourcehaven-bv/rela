package dataentry

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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
// whose COMPOSITION is sensitive even though the parts are readable, and to
// keep entries a user cannot use out of their sidebar.
//
// The deny response is the SAME 404 an unknown document name produces, so a
// caller cannot enumerate configured document names by probing. It fires
// before any renderer runs: content would be safe regardless, but an
// unauthorized caller must not be able to trigger an expensive Lua aggregation.
//
// A plain function rather than an *App method: it needs only the request's
// read gate and the document's own config, and App sits on its plimsoll load
// line (see the directive on the type).
func gateDocumentPermission(
	w http.ResponseWriter, r *http.Request, docCfg dataentryconfig.DocumentConfig,
) bool {
	if docCfg.Permission == "" {
		return true
	}
	if readGateFromContext(r.Context()).HoldsPermission(r.Context(), docCfg.Permission) {
		return true
	}
	writeV1Error(w, r, http.StatusNotFound, "document_not_found", "Document config not found", "")
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

	// Gate BEFORE rendering — see gateDocumentPermission.
	if !gateDocumentPermission(w, r, docCfg) {
		return
	}

	returnPath := isSafeReturnPath(r.URL.Query().Get("return_to"))

	result, err := a.documents.RenderStandalone(r.Context(), a.toDocumentRenderConfig(docName, &docCfg))
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
	// their real dependencies.
	writeV1JSON(w, http.StatusOK, v1.DocumentResponse{
		HTML:      RewriteDocumentLinks(result.HTML, returnPath, nil),
		Cached:    false,
		EntityIDs: []string{},
	})
}
