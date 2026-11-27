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
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
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
func (h *attachmentHandler) handleV1AttachmentRoute(
	w http.ResponseWriter, r *http.Request, typeName, plural, entityID, property string,
) {
	switch r.Method {
	case http.MethodPut, http.MethodPost:
		h.handleV1PutAttachment(w, r, typeName, plural, entityID, property)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// handleV1AttachmentFileRoute dispatches the per-file route
// /api/v1/{plural}/{id}/_attachments/{property}/{fileName}: GET downloads
// that file's bytes, DELETE detaches it. Reads inherit the entity's read
// permission, deletes inherit `update`.
func (h *attachmentHandler) handleV1AttachmentFileRoute(
	w http.ResponseWriter, r *http.Request, typeName, entityID, property, fileName string,
) {
	switch r.Method {
	case http.MethodGet:
		h.handleV1GetAttachment(w, r, typeName, entityID, property, fileName)
	case http.MethodDelete:
		h.handleV1DeleteAttachment(w, r, typeName, entityID, property, fileName)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// handleV1GetAttachment streams one file attached to a `file`-type
// property of one face:
//
//	GET /api/v1/{plural}/{address}/_attachments/{property}/{fileName}
//
// The address resolves through the gated resolver before any store lookup,
// and the property must be a declared, visible `file` property whose value on
// THIS face references the file. Every miss is the same 404, so a hidden face,
// property or another face's upload is indistinguishable from a missing one.
// fileName maps to a storage key through the face's own value, never used as
// a key or path itself.
//
//archplan:auth=authz
func (h *attachmentHandler) handleV1GetAttachment(
	w http.ResponseWriter, r *http.Request, typeName, addr, property, fileName string,
) {
	ctx := r.Context()
	s := h.schema()

	entity, found := readAddressedOr404(w, r, h.visible, typeName, addr)
	if !found {
		return
	}

	key, referenced := attachment.StorageKey(entity, property, fileName)
	if !isFileProperty(s, typeName, property) || h.isPropertyHidden(ctx, entity, property) || !referenced {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	rc, err := h.store.ReadFamilyAttachment(ctx, entity.ID, property, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return
		}
		slog.Warn("dataentry: read attachment failed",
			"err", err, "entity", entity.ID, "property", property)
		writeV1Error(w, r, http.StatusInternalServerError, "attachment_read_failed",
			"Reading the attachment failed", "check server logs")
		return
	}
	defer rc.Close()

	setHardenedDownloadHeaders(w.Header(), contentTypeForFilename(fileName), fileName)

	if _, err := io.Copy(w, rc); err != nil {
		slog.Warn("dataentry: streaming attachment failed",
			"err", err, "entity", entity.ID, "property", property)
	}
}

// handleV1PutAttachment uploads (or replaces) the file attached to a
// `file`-type property:
//
//	PUT|POST /api/v1/{plural}/{id}/_attachments/{property}   (multipart, field "file")
//	PUT|POST /api/v1/{plural}/{id}/_attachments/{property}?filename=x.png   (raw body)
//
// Writing an attachment mutates the owning entity's property, so it
// inherits the entity's `update` permission. The write is authorized
// up front (before any bytes are written) to avoid orphaning a file on a
// late deny — see attachment.Service.Attach's orphan note.
//
//archplan:auth=authz
func (h *attachmentHandler) handleV1PutAttachment(
	w http.ResponseWriter, r *http.Request, typeName, plural, entityID, property string,
) {
	r = h.withProvision(r)
	ctx := r.Context()
	s := h.schema()
	limit := maxAttachmentBytes(s)

	entity, ok := h.attachmentWritePreflight(w, r, s, typeName, entityID, property)
	if !ok {
		return
	}
	if filePropertyDef(s, entity.Type, property).IsImage() {
		limit = min(limit, metamodel.ImageMaxBytes)
	}

	releaseUpload, admitted := h.uploads.TryAcquire()
	if !admitted {
		writeAttachmentBusy(w, r, attachment.ErrBusy)
		return
	}
	defer releaseUpload()

	upload, ok := readUploadBody(w, r, limit)
	if !ok {
		return
	}
	defer upload.close()

	written, err := h.storeUpload(ctx, s, entity, property, upload.fileName, upload.body, limit)
	if errors.Is(err, errAttachmentServiceUnavailable) {
		slog.Warn("dataentry: attachment service unavailable", "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "attachment_write_failed",
			"Writing the attachment failed", "check server logs")
		return
	}
	if err != nil {
		writeAttachmentWriteError(w, r, limit, err)
		return
	}
	entity = written.Entity

	rels, visibleNeighbors, rerr := h.faceEdges.read(ctx, entity)
	if rerr != nil {
		writeGateError(w, r, rerr)
		return
	}
	result := h.serializer.forWireScoped(ctx, entity, rels, visibleNeighbors, s.Meta, plural)
	writeV1JSON(w, http.StatusOK, result)
}

// uploadBody is the file an upload request carries, however it was encoded.
type uploadBody struct {
	fileName string
	body     io.Reader
	close    func()
}

// readUploadBody extracts the file from an upload request and writes the
// error response itself when it cannot. It accepts multipart/form-data with
// the file in field "file", or any other Content-Type where the body IS the
// file and the name comes from `?filename=` or Content-Disposition.
// Both are capped at limit at ingress. The raw name is passed on unsanitized;
// the attachment service reduces it to a safe base name.
func readUploadBody(w http.ResponseWriter, r *http.Request, limit int64) (uploadBody, bool) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		return readMultipartUpload(w, r, limit)
	}

	name := r.URL.Query().Get("filename")
	if name == "" {
		if _, params, err := mime.ParseMediaType(r.Header.Get("Content-Disposition")); err == nil {
			name = params["filename"]
		}
	}
	if name == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_filename",
			"Missing file name", "pass ?filename= or a Content-Disposition filename with a raw upload body")
		return uploadBody{}, false
	}
	if ce := r.Header.Get("Content-Encoding"); ce != "" && !strings.EqualFold(ce, "identity") {
		writeV1Error(w, r, http.StatusUnsupportedMediaType, "unsupported_content_encoding",
			"Unsupported content encoding", "send the file uncompressed")
		return uploadBody{}, false
	}
	if r.ContentLength > limit {
		writeAttachmentTooLarge(w, r, limit)
		return uploadBody{}, false
	}
	body := http.MaxBytesReader(w, r.Body, limit)
	return uploadBody{fileName: name, body: body, close: func() { _ = body.Close() }}, true
}

// readMultipartUpload is the multipart arm of readUploadBody.
func readMultipartUpload(w http.ResponseWriter, r *http.Request, limit int64) (uploadBody, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, limit+maxAttachmentUploadHeadroom)
	if err := r.ParseMultipartForm(limit + maxAttachmentUploadHeadroom); err != nil {
		if isMaxBytesError(err) {
			writeAttachmentTooLarge(w, r, limit)
			return uploadBody{}, false
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_multipart",
			"Invalid multipart body", err.Error())
		return uploadBody{}, false
	}

	removeForm := func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		removeForm()
		writeV1Error(w, r, http.StatusBadRequest, "missing_file",
			"Missing form field \"file\"", "")
		return uploadBody{}, false
	}

	if header.Size > limit {
		_ = file.Close()
		removeForm()
		writeAttachmentTooLarge(w, r, limit)
		return uploadBody{}, false
	}
	return uploadBody{
		fileName: header.Filename,
		body:     file,
		close: func() {
			_ = file.Close()
			removeForm()
		},
	}, true
}

// auditRejectedUpload records an upload the attachment processor refused (a
// disallowed MIME type, or a failed or positive scan) as a security event.
// It uses the same op as the ACL denial (OpDeniedWrite), so one filter finds
// both; the Summary tells them apart. Only ErrRejected is recorded: size caps
// and I/O failures are not security events.
func (h *attachmentHandler) auditRejectedUpload(
	ctx context.Context, e *entityPkg.Entity, property, fileName string, err error,
) {
	if !errors.Is(err, attachment.ErrRejected) {
		return
	}
	h.audit().Record(audit.AttachmentRejected(ctx, e.Type, e.ID, property, attachment.DisplayName(fileName),
		attachment.RejectionReason(err)))
}

// writeAttachmentWriteError maps a Service.WriteAttachment failure to the
// right HTTP status: 413 (size), 409 (at capacity), 503 (another write to
// the property held it too long), 403 (ACL deny from the entity update, or a
// field write refused inside the write transaction), or 422 (validation /
// other).
func writeAttachmentWriteError(w http.ResponseWriter, r *http.Request, limit int64, err error) {
	if isAttachmentTooLarge(err) || isMaxBytesError(err) {
		writeAttachmentTooLarge(w, r, limit)
		return
	}
	if writeAttachmentBusy(w, r, err) {
		return
	}
	if errors.Is(err, attachment.ErrAtCapacity) {
		writeV1Error(w, r, http.StatusConflict, "attachment_limit",
			"Property already holds the maximum number of attachments", "")
		return
	}
	if errors.Is(err, attachment.ErrRejected) {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "attachment_rejected",
			"Attachment rejected", attachment.RejectionReason(err))
		return
	}
	if writeForbiddenIfACLDenied(w, r, err) || writeFieldWriteRefused(w, err) {
		return
	}
	slog.Warn("dataentry: attachment write failed", "err", err, "path", shapedPath(r))
	writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
		"Validation failed", err.Error())
}

// writeAttachmentBusy answers 503 with Retry-After when err is
// [attachment.ErrBusy], and reports whether it did.
func writeAttachmentBusy(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, attachment.ErrBusy) {
		return false
	}
	w.Header().Set("Retry-After", "1")
	writeV1Error(w, r, http.StatusServiceUnavailable, "attachment_busy",
		"Another write to this attachment property is in progress; retry", "")
	return true
}

// attachmentCmdTimeout bounds each external scan/transform command. Generous
// for a single synchronous upload of small-ish files; the runner also caps
// output size at the per-attachment limit.
const attachmentCmdTimeout = 60 * time.Second

// sandboxReporter is the one thing warnIfScanCannotRun needs from the command
// runner. Declared here, at the call site, rather than widening
// [attachment.CommandRunner] — which is deliberately narrow — and so that the
// warning's branches can be exercised with a stub on every platform instead of
// depending on whether the test host happens to have a working sandbox.
type sandboxReporter interface {
	// SandboxErr reports why commands cannot be confined, or nil when they can.
	SandboxErr() error
}

// warnIfScanCannotRun warns when the metamodel configures a virus scan that
// this host cannot run, because scanning is fail-closed and every upload to a
// scanned property will then be rejected.
// Nil: a nil runner is accepted; it means the constructor failed, and buildErr
// carries the reason, reported apart from sandbox problems.
func warnIfScanCannotRun(meta *metamodel.Metamodel, runner sandboxReporter, buildErr error) {
	if !metamodel.NewAttachmentPolicy(meta).HasConfiguredScan() {
		return
	}
	if runner == nil {
		slog.Warn("attachments: a virus scan is configured but the command runner could not "+
			"be built, so EVERY upload to a scanned property will be rejected",
			"err", buildErr, "docs", attachmentSecurityDoc)
		return
	}
	if err := runner.SandboxErr(); err != nil {
		slog.Warn("attachments: a virus scan is configured but no working sandbox is "+
			"available, so EVERY upload to a scanned property will be rejected",
			"err", err, "docs", attachmentSecurityDoc)
	}
}

// warnIfNoSandboxReadPaths warns at startup, per [cmdexec.Purpose], when
// commands of that purpose are configured and confined on Linux but the
// operator listed no read paths, so a scanner or converter cannot reach what
// it needs. It logs at most once per purpose per process, since a
// multi-tenant server builds one App per tenant. goos, paths and confined are
// parameters so every branch is testable on any host.
func warnIfNoSandboxReadPaths(meta *metamodel.Metamodel, docs map[string]dataentryconfig.DocumentConfig,
	goos string, paths map[cmdexec.Purpose][]string, confined bool,
) {
	if goos != "linux" || !confined {
		return
	}
	for _, purpose := range []cmdexec.Purpose{cmdexec.PurposeScan, cmdexec.PurposeTransform} {
		if len(paths[purpose]) > 0 || !commandsConfigured(meta, docs, purpose) {
			continue
		}
		if _, warned := noReadPathsWarning.LoadOrStore(purpose, true); warned {
			continue
		}
		slog.Warn("sandboxed commands are configured but their read paths are empty: "+
			"they can read only /usr, /bin, /sbin and /lib*, which is not enough "+
			"for clamdscan or a TeX-based PDF export",
			"purpose", purpose, "setting", purpose.EnvVar(), "docs", "docs/transforms.md#sandbox-read-paths")
	}
}

// noReadPathsWarning records which purposes [warnIfNoSandboxReadPaths] has
// warned about, keeping it to one line per purpose per process.
var noReadPathsWarning sync.Map

// commandsConfigured reports whether the project runs an external command of
// the given purpose. Scan: an attachment scan some property uses; a global
// scan_cmd that no property scans with never runs, so it does not count.
// Transform: an attachment transform step, an export transform, or a document
// rendered by a command.
func commandsConfigured(meta *metamodel.Metamodel, docs map[string]dataentryconfig.DocumentConfig,
	purpose cmdexec.Purpose,
) bool {
	if purpose == cmdexec.PurposeScan {
		return meta != nil && metamodel.NewAttachmentPolicy(meta).HasConfiguredScan()
	}
	if meta != nil {
		if len(meta.Transforms) > 0 {
			return true
		}
		for _, def := range meta.Entities {
			for _, prop := range def.Properties {
				for _, step := range prop.Transform {
					if prop.Type == metamodel.PropertyTypeFile && len(step.Cmd) > 0 {
						return true
					}
				}
			}
		}
	}
	for _, d := range docs {
		if len(d.Command) > 0 {
			return true
		}
	}
	return false
}

// attachmentCommands lists every attachment scan and transform command in the
// schema, empty ones omitted.
func attachmentCommands(meta *metamodel.Metamodel) [][]string {
	var out [][]string
	add := func(cmd []string) {
		if len(cmd) > 0 {
			out = append(out, cmd)
		}
	}
	if meta.Attachments != nil {
		add(meta.Attachments.ScanCmd)
	}
	for _, def := range meta.Entities {
		for _, prop := range def.Properties {
			if prop.Type != metamodel.PropertyTypeFile {
				continue
			}
			add(prop.ScanCmd)
			for _, step := range prop.Transform {
				add(step.Cmd)
			}
		}
	}
	return out
}

// attachmentSecurityDoc is the generated guide path, which is what an operator
// reading the log will look for on disk.
const attachmentSecurityDoc = "docs/attachment-security.md"

// probeAttachmentCommands checks, at startup, that every scan/transform binary
// referenced by the metamodel's file properties is resolvable on PATH, warning
// (never failing) for any that are missing — so an operator learns a typo or an
// uninstalled tool at boot rather than on the first upload.
func probeAttachmentCommands(meta *metamodel.Metamodel, runner *attachment.CmdRunner) {
	seen := map[string]bool{}
	for _, cmd := range attachmentCommands(meta) {
		if seen[cmd[0]] {
			continue
		}
		seen[cmd[0]] = true
		if err := runner.Probe(cmd); err != nil {
			slog.Warn("attachments: configured command not found", "binary", cmd[0], "err", err)
		}
	}
}

// aclAttachmentAuthorizer re-runs the preflight's `update` decision for the
// attachment service, under its attachment lock. A denial is audited like the
// preflight's and returned as an [acl.ForbiddenError], which the handlers
// already map to 403.
type aclAttachmentAuthorizer struct {
	acl   acl.ACL
	audit audit.Audit
}

func (a aclAttachmentAuthorizer) AuthorizeAttachmentWrite(ctx context.Context, e *entityPkg.Entity) error {
	decision := a.acl.AuthorizeWrite(ctx, translateVerb("update", e.Type, e.ID, e.Face))
	if decision.Allow {
		return nil
	}
	a.audit.Record(audit.AttachmentWriteDenied(ctx, e.Type, e.ID,
		decision.Reason, decision.RuleKind, decision.RuleID))
	return &acl.ForbiddenError{Decision: decision}
}

// attachmentService builds the shared attachment write-policy service from
// the App's dependencies and the GIVEN state snapshot. Cheap (a struct
// wrapper). Takes the snapshot explicitly so the service enforces the same
// metamodel the handler gated on — see "capture state once per operation".
func (h *attachmentHandler) attachmentService(s *Schema) (*attachment.Service, error) {
	return attachment.New(attachment.Deps{
		Store:         h.store,
		Meta:          s.Meta,
		EntityManager: h.owner,
		Locker:        h.owner,
		Authorizer:    aclAttachmentAuthorizer{acl: h.acl(), audit: h.audit()},
		Processor:     attachment.NewPolicyProcessor(s.Meta, h.runner()),
	})
}

// filePropertyDef returns the metamodel def for a property from the given
// snapshot (callers gate on isFileProperty first, so a file def is expected).
func filePropertyDef(s *Schema, typeName, property string) metamodel.PropertyDef {
	if def, ok := s.Meta.GetEntityDef(typeName); ok {
		return def.Properties[property]
	}
	return metamodel.PropertyDef{}
}

// handleV1DeleteAttachment detaches one file from a `file`-type property:
//
//	DELETE /api/v1/{plural}/{id}/_attachments/{property}/{fileName}
//
// Inherits the face's `update` permission (a deny is handled up front in
// attachmentWritePreflight, before anything is touched). The name is
// removed from the face's value, and the bytes go when no other face
// references them. Idempotent: deleting a name the face does not reference
// changes nothing and returns 204.
//
//archplan:auth=authz
func (h *attachmentHandler) handleV1DeleteAttachment(
	w http.ResponseWriter, r *http.Request, typeName, entityID, property, fileName string,
) {
	r = h.withProvision(r)
	ctx := r.Context()
	s := h.schema()

	entity, ok := h.attachmentWritePreflight(w, r, s, typeName, entityID, property)
	if !ok {
		return
	}

	svc, err := h.attachmentService(s)
	if err != nil {
		slog.Warn("dataentry: attachment service unavailable", "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "attachment_delete_failed",
			"Deleting the attachment failed", "check server logs")
		return
	}
	propDef := filePropertyDef(s, typeName, property)
	if err := svc.DeleteAttachment(ctx, entity, propDef, property, fileName); err != nil {
		if writeAttachmentBusy(w, r, err) || writeForbiddenIfACLDenied(w, r, err) || writeFieldWriteRefused(w, err) {
			return
		}
		slog.Warn("dataentry: delete attachment failed", "err", err, "path", shapedPath(r))
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			"Validation failed", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// errEntityLocked is the refusal [attachmentHandler.attachmentWriteCheck]
// reports for a git-crypt encrypted entity.
var errEntityLocked = errors.New("cannot edit an inaccessible entity")

// attachmentWritePreflight runs [attachmentHandler.attachmentWriteCheck] and
// writes the response for a refusal. Returns the raw face and true when the
// write may proceed.
func (h *attachmentHandler) attachmentWritePreflight(
	w http.ResponseWriter, r *http.Request, s *Schema, typeName, addr, property string,
) (*entityPkg.Entity, bool) {
	entity, err := h.attachmentWriteCheck(r.Context(), s, typeName, addr, property)
	if err == nil {
		return entity, true
	}
	var denial AffordanceDenialError
	switch {
	case errors.Is(err, errEntityLocked):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "encrypted_inaccessible",
			"Cannot edit an inaccessible entity", "File is git-crypt encrypted; run `git-crypt unlock` first.")
	case errors.As(err, &denial):
		writeAffordanceDenialError(w, denial)
	case writeForbiddenIfACLDenied(w, r, err):
	default:
		writeWriteTargetError(w, r, err)
	}
	return nil, false
}

// attachmentWriteCheck is the shared front matter for an attachment write,
// used by the upload route and rela.attach_render. It resolves the write
// target face, checks the property is a declared `file` property the caller
// can see, rejects a locked entity, and authorizes `update` up front so a deny
// never reaches the store. Every miss, including a hidden property, is
// [errWriteTargetNotFound]; other errors are [errEntityLocked],
// [*acl.ForbiddenError], [AffordanceDenialError], the ambiguous-address error
// or a gate error. Both denials are audited here.
func (h *attachmentHandler) attachmentWriteCheck(
	ctx context.Context, s *Schema, typeName, addr, property string,
) (*entityPkg.Entity, error) {
	entity, err := h.visible.writeTarget(ctx, typeName, addr)
	if err != nil {
		return nil, err
	}
	if !isFileProperty(s, typeName, property) || h.isPropertyHidden(ctx, entity, property) {
		return nil, errWriteTargetNotFound
	}
	if entity.IsLocked() {
		return nil, errEntityLocked
	}

	decision := h.acl().AuthorizeWrite(ctx, translateVerb("update", entity.Type, entity.ID, entity.Face))
	if !decision.Allow {
		h.audit().Record(audit.AttachmentWriteDenied(ctx, entity.Type, entity.ID,
			decision.Reason, decision.RuleKind, decision.RuleID))
		return nil, &acl.ForbiddenError{Decision: decision}
	}
	if denial := h.affordances.validateFieldWrite(ctx, entity, map[string]any{property: nil}, nil); denial != nil {
		reason := denial.Reason
		if denial.Attribution != "" {
			reason += " attribution=" + denial.Attribution
		}
		h.audit().Record(audit.AttachmentWriteDenied(ctx, entity.Type, entity.ID,
			reason, "affordance", denial.RuleID()))
		return nil, *denial
	}
	return entity, nil
}

// storeUpload is the write half of an upload, shared by the PUT route and
// rela.attach_render: it caps body at limit, writes it through the shared
// attachment service (scan, MIME policy, count limit, entitymanager stamp),
// and audits a processor rejection. entity has passed
// [attachmentHandler.attachmentWriteCheck]; the caller holds an upload slot.
func (h *attachmentHandler) storeUpload(
	ctx context.Context, s *Schema, entity *entityPkg.Entity, property, fileName string, body io.Reader, limit int64,
) (*attachment.Result, error) {
	svc, err := h.attachmentService(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errAttachmentServiceUnavailable, err)
	}
	propDef := filePropertyDef(s, entity.Type, property)
	written, err := svc.WriteAttachment(ctx, entity, propDef, property, fileName,
		store.CapAttachmentReader(body, limit))
	if err != nil {
		h.auditRejectedUpload(ctx, entity, property, fileName, err)
		return nil, err
	}
	return written, nil
}

// errAttachmentServiceUnavailable marks a failure to build the attachment
// service, a server fault rather than anything about the upload.
var errAttachmentServiceUnavailable = errors.New("attachment service unavailable")

// maxAttachmentBytes returns the effective per-attachment cap for the
// given app state: the configured override (or the product default),
// clamped to the store backstop. Clamping guarantees the 413 detail never
// promises a ceiling higher than the store will actually accept — a
// misconfigured `max_attachment_bytes` above store.MaxAttachmentBytes
// can't make the error message lie.
func maxAttachmentBytes(s *Schema) int64 {
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
func writeAttachmentTooLarge(w http.ResponseWriter, r *http.Request, limit int64) {
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
func isFileProperty(s *Schema, typeName, property string) bool {
	def, ok := s.Meta.GetEntityDef(typeName)
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
func (h *attachmentHandler) isPropertyHidden(ctx context.Context, e *entityPkg.Entity, property string) bool {
	return !h.fields().FieldVerdicts(ctx, e).IsVisible(property)
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

// setHardenedDownloadHeaders is the shared hardening for endpoints that serve
// user-influenced bytes as a download (attachments, view exports): force a
// download rather than inline rendering (Content-Disposition: attachment),
// never let the browser sniff a different (e.g. text/html) type, and sandbox
// any active content — so an SVG/HTML payload can't execute as stored XSS in
// the app's origin. The filename is sanitized with [safeAttachmentFilename].
func setHardenedDownloadHeaders(hdr http.Header, contentType, filename string) {
	hdr.Set("Content-Type", contentType)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	hdr.Set("Content-Disposition", `attachment; filename="`+safeAttachmentFilename(filename)+`"`)
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
