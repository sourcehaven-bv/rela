package dataentry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DefaultMaxAttachmentBytes is the product-wide default cap on a single
// attachment, applied at the HTTP upload ingress. It is generously sized
// for the expected use (screenshots, PDFs, office docs), not media. A
// deployment can override it via dataentryconfig; the store backends also
// enforce their own backstop guard so no path is ever unbounded.
const DefaultMaxAttachmentBytes = 64 << 20 // 64 MiB

// maxAttachmentUploadHeadroom is added to the content cap for the
// multipart envelope (boundaries, headers), mirroring the theme upload
// path's maxLogoUploadBytes margin.
const maxAttachmentUploadHeadroom = 16 * 1024

// handleV1AttachmentRoute dispatches the property-level route
// /api/v1/{plural}/{id}/_attachments/{property}: PUT/POST uploads a file
// (appending up to the property's `max`), GET is not used at this level
// (the entity's `_attachments` map already lists the files; downloads use
// the per-file route below). Writes inherit the entity's `update`
// permission.
func (a *App) handleV1AttachmentRoute(w http.ResponseWriter, r *http.Request, typeName, plural, entityID, property string) {
	switch r.Method {
	case http.MethodPut, http.MethodPost:
		a.handleV1PutAttachment(w, r, typeName, plural, entityID, property)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// handleV1AttachmentFileRoute dispatches the per-file route
// /api/v1/{plural}/{id}/_attachments/{property}/{fileName}: GET downloads
// that file's bytes, DELETE detaches it. Reads inherit the entity's read
// permission, deletes inherit `update`.
func (a *App) handleV1AttachmentFileRoute(w http.ResponseWriter, r *http.Request, typeName, entityID, property, fileName string) {
	switch r.Method {
	case http.MethodGet:
		a.handleV1GetAttachment(w, r, typeName, entityID, property, fileName)
	case http.MethodDelete:
		a.handleV1DeleteAttachment(w, r, typeName, entityID, property, fileName)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// handleV1GetAttachment streams one file attached to a `file`-type
// property of an entity:
//
//	GET /api/v1/{plural}/{id}/_attachments/{property}/{fileName}
//
// Access inherits the owning entity's read permission — a caller who
// cannot read the entity cannot read its attachment. The gate runs
// BEFORE any store lookup so a hidden id and a nonexistent id are
// indistinguishable (404, no body difference, no timing side channel —
// same RR-NGMI invariant as handleV1GetEntity).
//
// The bytes are resolved from (entityID, property, fileName) only; the
// path string stored in the entity's frontmatter is never parsed or
// trusted, so a renamed entity resolves correctly by its current id and
// there is no caller-supplied-path traversal surface. The fileName comes
// from the URL but is only ever a store key (never a filesystem path the
// handler builds), and the store's ValidateFileName rejects separators.
func (a *App) handleV1GetAttachment(w http.ResponseWriter, r *http.Request, typeName, entityID, property, fileName string) {
	ctx := r.Context()

	// ACL gate first — before any store access (see handleV1GetEntity).
	if !a.gateReadOrNotFound(w, r, typeName, entityID) {
		return
	}

	entity, found := a.getEntity(ctx, entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	// The property must be a declared `file`-type property on this entity
	// type, and visible to this viewer. Anything else 404s — we never reveal
	// whether some other (or hidden) property or path exists.
	if !a.isFileProperty(typeName, property) || a.isPropertyHidden(ctx, entity, property) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	rc, err := a.store.ReadAttachment(ctx, entityID, property, fileName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return
		}
		// Don't leak backend error strings (table/column/path names) —
		// same rationale as writeGateError. Log server-side, 500 client-side.
		slog.Warn("dataentry: read attachment failed",
			"err", err, "entity", entityID, "property", property)
		writeV1Error(w, r, http.StatusInternalServerError, "attachment_read_failed",
			"Reading the attachment failed", "check server logs")
		return
	}
	defer rc.Close()

	// Serve user-supplied bytes defensively: never let the browser sniff a
	// different (e.g. text/html) type, sandbox any active content, and
	// send Content-Disposition with a sanitized filename so an SVG/HTML
	// payload can't execute as stored XSS in the app's origin. Mirrors the
	// theme-logo serve path.
	h := w.Header()
	h.Set("Content-Type", contentTypeForFilename(fileName))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Content-Disposition", `inline; filename="`+safeAttachmentFilename(fileName)+`"`)

	if _, err := io.Copy(w, rc); err != nil {
		// Headers (and likely some bytes) are already written; we can't
		// change the status now. Log and move on.
		slog.Warn("dataentry: streaming attachment failed",
			"err", err, "entity", entityID, "property", property)
	}
}

// handleV1PutAttachment uploads (or replaces) the file attached to a
// `file`-type property:
//
//	PUT|POST /api/v1/{plural}/{id}/_attachments/{property}   (multipart, field "file")
//
// Writing an attachment mutates the owning entity's property, so it
// inherits the entity's `update` permission. The write is authorized
// up front (before any bytes are written) to avoid orphaning a file on a
// late deny — see attachment.Service.Attach's orphan note.
func (a *App) handleV1PutAttachment(w http.ResponseWriter, r *http.Request, typeName, plural, entityID, property string) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	ctx := r.Context()
	// Capture the state snapshot once: the cap the handler enforces and the
	// cap the attachment service enforces must come from the same metamodel
	// (CLAUDE.md "capture state once per operation").
	s := a.State()
	limit := maxAttachmentBytes(s)

	entity, ok := a.attachmentWritePreflight(w, r, typeName, entityID, property)
	if !ok {
		return
	}

	// Cap the request body at ingress: MaxBytesReader makes ParseMultipartForm
	// and the FormFile read fail with *http.MaxBytesError once the limit is
	// crossed, which we map to 413. The store backends enforce their own cap
	// as a backstop, but this rejects an oversize upload before buffering it.
	r.Body = http.MaxBytesReader(w, r.Body, limit+maxAttachmentUploadHeadroom)
	if err := r.ParseMultipartForm(limit + maxAttachmentUploadHeadroom); err != nil {
		if isMaxBytesError(err) {
			a.writeAttachmentTooLarge(w, r, limit)
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_multipart",
			"Invalid multipart body", err.Error())
		return
	}

	// Clean up any on-disk temp files the multipart parser spilled past its
	// in-memory threshold. Go's server removes them when the request body
	// closes, but for a 64 MiB upload path being explicit is cheap insurance.
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "missing_file",
			"Missing form field \"file\"", "")
		return
	}
	defer file.Close()

	// The ingress MaxBytesReader bounds the whole multipart request, but its
	// envelope headroom means it doesn't precisely cap the file *content*.
	// header.Size is the part's declared length — reject early when it's over.
	if header.Size > limit {
		a.writeAttachmentTooLarge(w, r, limit)
		return
	}

	// Delegate the cap / suffix / write-order / re-stamp policy to the
	// shared attachment service so the HTTP and CLI paths apply identical
	// rules (and the same data-loss-safe attach-then-delete ordering).
	// Cap the reader at the configured limit (which clamps ≤ the store
	// backstop) so an under-declared multipart part still can't exceed it.
	svc, err := a.attachmentService(s)
	if err != nil {
		slog.Warn("dataentry: attachment service unavailable", "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "attachment_write_failed",
			"Writing the attachment failed", "check server logs")
		return
	}
	propDef := filePropertyDef(s, typeName, property)
	capped := store.CapAttachmentReader(file, limit)
	if _, err := svc.WriteAttachment(ctx, entity, propDef, property, header.Filename, capped); err != nil {
		a.writeAttachmentWriteError(w, r, limit, err)
		return
	}

	result := a.serializeEntityForWire(ctx, entity, plural, true)
	writeV1JSON(w, http.StatusOK, result)
}

// writeAttachmentWriteError maps a Service.WriteAttachment failure to the
// right HTTP status: 413 (size), 409 (at capacity), 403 (ACL deny from the
// entity update), or 422 (validation / other).
func (a *App) writeAttachmentWriteError(w http.ResponseWriter, r *http.Request, limit int64, err error) {
	if isAttachmentTooLarge(err) {
		a.writeAttachmentTooLarge(w, r, limit)
		return
	}
	if errors.Is(err, attachment.ErrAtCapacity) {
		writeV1Error(w, r, http.StatusConflict, "attachment_limit",
			"Property already holds the maximum number of attachments", "")
		return
	}
	if writeForbiddenIfACLDenied(w, err) {
		return
	}
	slog.Warn("dataentry: attachment write failed", "err", err, "path", r.URL.Path)
	writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
		"Validation failed", err.Error())
}

// attachmentService builds the shared attachment write-policy service from
// the App's dependencies and the GIVEN state snapshot. Cheap (a struct
// wrapper). Takes the snapshot explicitly so the service enforces the same
// metamodel the handler gated on — see "capture state once per operation".
func (a *App) attachmentService(s *AppState) (*attachment.Service, error) {
	return attachment.New(attachment.Deps{
		Store:         a.store,
		Meta:          s.Meta,
		EntityManager: a.entityManager,
	})
}

// filePropertyDef returns the metamodel def for a property from the given
// snapshot (callers gate on isFileProperty first, so a file def is expected).
func filePropertyDef(s *AppState, typeName, property string) metamodel.PropertyDef {
	if def, ok := s.Meta.GetEntityDef(typeName); ok {
		return def.Properties[property]
	}
	return metamodel.PropertyDef{}
}

// handleV1DeleteAttachment detaches one file from a `file`-type property:
//
//	DELETE /api/v1/{plural}/{id}/_attachments/{property}/{fileName}
//
// Inherits the entity's `update` permission (a deny is handled up front in
// attachmentWritePreflight, before anything is touched). The bytes are
// removed, then the property is re-stamped from the store's remaining
// files and persisted. Idempotent: deleting a missing file still
// re-stamps and returns 204.
func (a *App) handleV1DeleteAttachment(w http.ResponseWriter, r *http.Request, typeName, entityID, property, fileName string) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	ctx := r.Context()
	s := a.State()

	entity, ok := a.attachmentWritePreflight(w, r, typeName, entityID, property)
	if !ok {
		return
	}

	svc, err := a.attachmentService(s)
	if err != nil {
		slog.Warn("dataentry: attachment service unavailable", "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "attachment_delete_failed",
			"Deleting the attachment failed", "check server logs")
		return
	}
	propDef := filePropertyDef(s, typeName, property)
	if err := svc.DeleteAttachment(ctx, entity, propDef, property, fileName); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		slog.Warn("dataentry: delete attachment failed", "err", err, "path", r.URL.Path)
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			"Validation failed", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// attachmentWritePreflight runs the shared front matter for an attachment
// write: read-gate (uniform 404), load the entity, validate the property
// is a declared `file` type, reject a locked (inaccessible) entity, and
// authorize the `update` write UP FRONT so a deny never reaches the store.
// Returns the loaded entity and true when the write may proceed.
func (a *App) attachmentWritePreflight(w http.ResponseWriter, r *http.Request, typeName, entityID, property string) (*entityPkg.Entity, bool) {
	ctx := r.Context()

	// Read-gate first: a hidden or nonexistent id yields a uniform 404
	// (RR-NGMI), the same as the read path.
	if !a.gateReadOrNotFound(w, r, typeName, entityID) {
		return nil, false
	}

	entity, found := a.getEntity(ctx, entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return nil, false
	}
	if !a.isFileProperty(typeName, property) || a.isPropertyHidden(ctx, entity, property) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return nil, false
	}
	if entity.IsLocked() {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "encrypted_inaccessible",
			"Cannot edit an inaccessible entity", "File is git-crypt encrypted; run `git-crypt unlock` first.")
		return nil, false
	}

	// Authorize the `update` write up front (mirrors authorizeConflictResolve)
	// so an ACL deny happens before any bytes are written to the store.
	decision := a.acl.AuthorizeWrite(ctx, translateVerb("update", entity.Type, entity.ID))
	if !decision.Allow {
		a.auditSink.Record(audit.Record{
			Time:        time.Now().UTC(),
			Op:          audit.OpDeniedWrite,
			Subject:     &audit.Subject{Kind: "entity", Type: entity.Type, ID: entity.ID},
			Principal:   principal.From(ctx),
			TriggeredBy: audit.TriggeredByFrom(ctx),
			Summary: fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s op=attachment-write)",
				decision.Reason, decision.RuleKind, decision.RuleID),
		})
		writeForbiddenIfACLDenied(w, &acl.ForbiddenError{Decision: decision})
		return nil, false
	}
	return entity, true
}

// maxAttachmentBytes returns the effective per-attachment cap for the
// given app state: the configured override (or the product default),
// clamped to the store backstop. Clamping guarantees the 413 detail never
// promises a ceiling higher than the store will actually accept — a
// misconfigured `max_attachment_bytes` above store.MaxAttachmentBytes
// can't make the error message lie.
func maxAttachmentBytes(s *AppState) int64 {
	limit := int64(DefaultMaxAttachmentBytes)
	if n := s.Cfg.App.MaxAttachmentBytes; n > 0 {
		limit = n
	}
	if limit > store.MaxAttachmentBytes {
		limit = store.MaxAttachmentBytes
	}
	return limit
}

// writeAttachmentTooLarge emits the 413 problem+json body for an
// over-cap upload.
func (a *App) writeAttachmentTooLarge(w http.ResponseWriter, r *http.Request, limit int64) {
	writeV1Error(w, r, http.StatusRequestEntityTooLarge, "attachment_too_large",
		"Attachment too large", fmt.Sprintf("maximum size is %d bytes", limit))
}

// isMaxBytesError reports whether err is (or wraps) an *http.MaxBytesError.
func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// isAttachmentTooLarge reports whether a store AttachFile error is the
// backend's own size-cap rejection (the per-store backstop).
func isAttachmentTooLarge(err error) bool {
	return errors.Is(err, store.ErrAttachmentTooLarge)
}

// isFileProperty reports whether property is a declared `file`-type
// property on the given entity type.
func (a *App) isFileProperty(typeName, property string) bool {
	def, ok := a.State().Meta.GetEntityDef(typeName)
	if !ok {
		return false
	}
	pd, ok := def.Properties[property]
	return ok && pd.Type == metamodel.PropertyTypeFile
}

// isPropertyHidden reports whether property is hidden from the current
// viewer by field-visibility policy. Attachment read/write endpoints 404
// on a hidden property so its files (and download URLs) never leak — the
// same boundary stripHiddenProperties / `_fields` enforce on the entity
// response.
func (a *App) isPropertyHidden(ctx context.Context, e *entityPkg.Entity, property string) bool {
	return !a.fieldResolver.FieldVerdicts(ctx, e).IsVisible(property)
}

// contentTypeForFilename infers a MIME type from a filename extension,
// defaulting to application/octet-stream (browsers prompt a download for
// that, the right default for an unknown type). The store does not
// persist content type on every backend, so we always derive it here.
func contentTypeForFilename(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return "application/octet-stream"
	}
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	return "application/octet-stream"
}

// safeAttachmentFilename strips characters that could break out of the
// Content-Disposition filename token (quotes, control chars, path
// separators) while preserving the extension so the downloaded file
// keeps a sensible name. The stem and extension are sanitized separately
// with the theme filename allowlist (which collapses `.`), then rejoined
// with a single dot.
func safeAttachmentFilename(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	cleanStem := strings.Trim(unsafeFilenameRe.ReplaceAllString(stem, "_"), "_")
	if cleanStem == "" {
		cleanStem = "attachment"
	}
	cleanExt := strings.Trim(unsafeFilenameRe.ReplaceAllString(strings.TrimPrefix(ext, "."), "_"), "_")
	if cleanExt == "" {
		return cleanStem
	}
	return cleanStem + "." + cleanExt
}
