package dataentry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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
// document the caller has been authorized to render, plus the entry entity's
// ADDRESS (`ID` or `ID@face`, "" for a standalone document): the exact row the
// gates cleared, which is the row the renderer reads.
//
// It deliberately carries nothing else. Everything the HTML path additionally
// needs — the return_to rewrite target, the refresh flag, the disk cache — is
// specific to producing HTML and stays in that handler. See the note on
// resolveAnchoredDocument about GetCached in particular.
type resolvedDocument struct {
	cfg     documentRenderConfig
	entryID string
}

// resolveStandaloneDocument runs [standaloneDocumentGates] and writes the
// response for a denial. On any denial it returns ok=false.
func resolveStandaloneDocument(
	a *App, w http.ResponseWriter, r *http.Request, docName string,
) (resolvedDocument, bool) {
	resolved, err := standaloneDocumentGates(r.Context(), a, a.State(), docName)
	if err != nil {
		writeRequestDenial(w, r, err)
		return resolvedDocument{}, false
	}
	return resolved, true
}

// standaloneDocumentGates runs the full gate chain for a standalone document
// and returns its render config. A denial is a [*denialError]; the render
// route and rela.attach_render both run this one chain.
//
// This exists so the render and export paths cannot drift. The gates are
// ORDERED and the order is load-bearing; two copies of an ordered security
// chain is how one copy quietly loses a gate. s is the caller's snapshot,
// so the gates read the config the caller resolved everything else from.
func standaloneDocumentGates(
	ctx context.Context, a *App, s *Schema, docName string,
) (resolvedDocument, error) {
	if docName == "" {
		return resolvedDocument{}, deny(http.StatusBadRequest, "invalid_path",
			"Path must be /_documents/{docName} or /_documents/{docName}/{entityId}")
	}

	if !isSafePathSegment(docName) {
		return resolvedDocument{}, deny(http.StatusBadRequest, "invalid_path", "Path segment contains forbidden characters")
	}

	docCfg, ok := s.Cfg.Documents[docName]
	if !ok {
		return resolvedDocument{}, deny(http.StatusNotFound, "document_not_found", "Document config not found")
	}

	if !docCfg.IsStandalone() {
		return resolvedDocument{}, deny(http.StatusBadRequest, "document_kind_mismatch",
			documentKindMismatchAnchored(docName, docCfg))
	}

	if err := gateDocumentPermission(ctx, docName, docCfg); err != nil {
		return resolvedDocument{}, err
	}
	if err := gateElevatedDocument(ctx, a.acl, docName, docCfg); err != nil {
		return resolvedDocument{}, err
	}

	return resolvedDocument{cfg: a.toDocumentRenderConfig(docName, &docCfg)}, nil
}

// resolveAnchoredDocument runs [anchoredDocumentGates] and writes the
// response for a denial. On any denial it returns ok=false.
func resolveAnchoredDocument(
	a *App, w http.ResponseWriter, r *http.Request, docName, entityID string,
) (resolvedDocument, bool) {
	resolved, err := anchoredDocumentGates(r.Context(), a, a.State(), docName, entityID)
	if err != nil {
		writeRequestDenial(w, r, err)
		return resolvedDocument{}, false
	}
	return resolved, true
}

// anchoredDocumentGates runs the full gate chain for an entity-anchored
// document and returns its render config plus the entry entity ID. A denial
// is a [*denialError] or a gate error. The ACL read gate runs BEFORE the store
// read, so a hidden id and a missing one are indistinguishable. The type
// check runs below the read gate, so a denial never reveals the type. It does
// not touch the document cache, whose key has no principal.
func anchoredDocumentGates(
	ctx context.Context, a *App, s *Schema, docName, entityID string,
) (resolvedDocument, error) {
	ref, refErr := entity.ParseRef(entityID)
	if !isSafePathSegment(docName) || !isSafeStateRefSegment(entityID) || refErr != nil {
		return resolvedDocument{}, deny(http.StatusBadRequest, "invalid_path", "Path segment contains forbidden characters")
	}

	docCfg, ok := s.Cfg.Documents[docName]
	if !ok {
		return resolvedDocument{}, deny(http.StatusNotFound, "document_not_found", "Document config not found")
	}

	if docCfg.IsStandalone() {
		return resolvedDocument{}, deny(http.StatusBadRequest, "document_kind_mismatch",
			documentKindMismatchStandalone(docName))
	}

	typ := a.visibleReader.storedType(ctx, ref.ID)
	if typ == "" {
		typ = docCfg.EntityType
	}
	ent, found, gateErr := a.visibleReader.addressRef(ctx, typ, ref)
	if gateErr != nil {
		return resolvedDocument{}, gateErr
	}
	if !found {
		return resolvedDocument{}, deny(http.StatusNotFound, "not_found", entityNotFoundTitle)
	}

	if err := gateDocumentPermission(ctx, docName, docCfg); err != nil {
		return resolvedDocument{}, err
	}

	if err := gateElevatedDocument(ctx, a.acl, docName, docCfg); err != nil {
		return resolvedDocument{}, err
	}

	if ent.Type != docCfg.EntityType {
		return resolvedDocument{}, deny(http.StatusBadRequest, "entity_type_mismatch",
			documentTypeMismatch(docName, docCfg.EntityType, entityID, ent.Type))
	}

	return resolvedDocument{cfg: a.toDocumentRenderConfig(docName, &docCfg), entryID: ent.Ref().String()}, nil
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
		return
	}

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

	a.export.convertAndWrite(w, r, reg, name, renderer, exportBaseName(docName, resolved.entryID),
		"document", docName)
}

// exportBaseName is the download filename stem: the document name for a
// standalone export, "<doc>-<address>" when anchored to an entity, using the
// address of the row that was rendered. The document name has passed
// isSafePathSegment, the address was built from a stored row, and
// safeAttachmentFilename sanitizes the result again downstream.
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
