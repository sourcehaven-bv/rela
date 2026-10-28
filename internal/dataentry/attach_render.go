package dataentry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/transform"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// markdownFormat is the attach_render format that stores the rendered
// markdown itself. A registered transform of the same name wins.
const markdownFormat = "md"

// markdownMIME is the content type of [markdownFormat].
const markdownMIME = "text/markdown"

// attachRenderer is rela.attach_render (TKT-1TS55R): it renders a document
// or a list as the ctx principal, converts the markdown with a registered
// transform, and stores the result on a file property like an upload.
//
// It runs the chains the HTTP routes run, not copies of them: the document
// gates of the render and export routes ([standaloneDocumentGates],
// [anchoredDocumentGates]), the list export pipeline
// ([exportHandler.listExportRenderer]), the upload preflight
// ([attachmentHandler.attachmentWriteCheck]) and write
// ([attachmentHandler.storeUpload]), and the shared transform engine. What it
// adds is binding the request state a script context lacks (the read gate
// and the world), and the audit record that ties the stored bytes to their
// source.
//
// A type of its own rather than App methods: App is at its plimsoll method
// cap.
type attachRenderer struct {
	a *App
}

// newAttachRenderer returns the renderer over a.
//
// Nil: a nil App, or one without its export or attachment handler, is
// rejected.
func newAttachRenderer(a *App) (attachRenderer, error) {
	switch {
	case a == nil:
		return attachRenderer{}, errors.New("dataentry: newAttachRenderer: app is required")
	case a.export == nil || a.export.engine == nil:
		return attachRenderer{}, errors.New("dataentry: newAttachRenderer: export handler is required")
	case a.attachments == nil || a.attachments.uploads == nil:
		return attachRenderer{}, errors.New("dataentry: newAttachRenderer: attachment handler is required")
	case a.documents == nil:
		return attachRenderer{}, errors.New("dataentry: newAttachRenderer: document service is required")
	}
	return attachRenderer{a: a}, nil
}

// AttachRenderer returns rela.attach_render over a, for a host that runs
// scripts outside the data-entry app's own actions (schedules and background
// automations, see appbuild.SetAttachRenderer).
func AttachRenderer(a *App) (lua.AttachRenderer, error) {
	return newAttachRenderer(a)
}

// actionAttachRenderer is the renderer an action script gets, or nil when a
// collaborator is missing, which leaves the binding absent. The missing
// collaborator is logged: it is a wiring fault, not a script error.
func actionAttachRenderer(a *App) lua.AttachRenderer {
	ar, err := newAttachRenderer(a)
	if err != nil {
		slog.Warn("attach_render unavailable to actions", "err", err)
		return nil
	}
	return ar
}

// AttachRender implements [lua.AttachRenderer]. Errors are written for the
// script author: a refusal says what was refused, an internal failure says
// only that it failed, and its cause goes to the log.
func (ar attachRenderer) AttachRender(ctx context.Context, req lua.AttachRenderRequest) (lua.AttachedFile, error) {
	ctx, err := ar.bindRequestState(ctx)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	s := ar.a.State()

	conv, err := resolveRenderFormat(s, req.Format)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	fileName, err := renderFileName(req.Filename, req.Format, conv.produces)
	if err != nil {
		return lua.AttachedFile{}, err
	}

	// The write is checked before the render, so a principal who may not
	// write the property cannot make the server run a render.
	target, err := ar.writeTarget(ctx, s, req.Entity, req.Property)
	if err != nil {
		return lua.AttachedFile{}, err
	}

	// The upload slot is taken before the render, as the HTTP upload takes
	// it before reading the body: the rendered bytes are held in memory, and
	// the slot bounds how many such buffers exist at once.
	release, admitted := ar.a.attachments.uploads.TryAcquire()
	if !admitted {
		return lua.AttachedFile{}, errors.New("attach_render: too many uploads in progress; retry later")
	}
	defer release()

	renderer, source, err := ar.renderer(ctx, s, req)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	data, mimeType, err := ar.convert(ctx, conv, renderer)
	if err != nil {
		return lua.AttachedFile{}, err
	}

	written, err := ar.store(ctx, s, target, req.Property, fileName, data)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	source += fmt.Sprintf(" format=%q", req.Format)
	// The subject is the full address, so a faced type records which face
	// got the file.
	ar.a.auditSink.Record(audit.AttachRendered(ctx, target.Type, target.Ref().String(), req.Property,
		written.FileName, written.SHA256, written.Size, source))
	return lua.AttachedFile{
		Name: written.FileName, Size: written.Size, SHA256: written.SHA256, MIME: mimeType,
	}, nil
}

// bindRequestState gives ctx what an API request carries and a script
// context may lack.
//
// The read gate first, and it fails closed. Outside an HTTP request nothing
// attached one, and without it [readGateFromContext] would hand back the
// allow-all gate and every read below would be ungated. Under a declarative
// policy the gate is built from the Request bound on ctx; with none bound the
// call is refused. Re-attaching on the action path, where the middleware
// already attached one, builds the same gate from the same Request.
//
// Then the world: a context with none reads in the declared default world, as
// a source with no world does on the dashboard (see nextActionSourceWorld).
func (ar attachRenderer) bindRequestState(ctx context.Context) (context.Context, error) {
	if _, declarative := ar.a.acl.(*acl.Declarative); declarative {
		bound, err := attachBoundReadGate(withACLEnforced(ctx), ar.a.acl)
		if err != nil {
			slog.Warn("attach_render: no identity to read as", "err", err)
			return ctx, errors.New("attach_render: no identity is bound to this script")
		}
		ctx = bound
	}
	if _, bound := ctx.Value(worldCtxKey{}).(worldHandle); bound {
		return ctx, nil
	}
	ctx, ok, err := nextActionSourceWorld(ctx, ar.a.worlds, effectiveDefaultWorld(ar.a),
		dataentryconfig.NextActionSource{})
	if err != nil || !ok {
		slog.Warn("attach_render: default world unavailable", "err", err)
		return ctx, errors.New("attach_render: the default world is not available")
	}
	return ctx, nil
}

// renderConversion is how the markdown becomes the stored bytes: through a
// registered transform, or as is.
type renderConversion struct {
	reg      transform.Registry
	name     string // "" for markdown as is
	produces string
}

// resolveRenderFormat resolves format against the transforms registered in
// s. A request may only name a transform, never a command.
func resolveRenderFormat(s *Schema, format string) (renderConversion, error) {
	reg := transform.RegistryFromMetamodel(s.Meta)
	if def, ok := reg[format]; ok {
		return renderConversion{reg: reg, name: format, produces: def.Produces}, nil
	}
	if format == markdownFormat {
		return renderConversion{produces: markdownMIME}, nil
	}
	return renderConversion{}, fmt.Errorf("attach_render: unknown format %q: name a registered transform or %q",
		format, markdownFormat)
}

// renderFileName checks the requested name against the format's output: a
// name without an extension gets the format's, and a name whose extension
// belongs to another type is refused, so a PDF is never stored as
// `report.md`. A suffix that names no known type counts as part of the
// name. A format whose type has no known extension gets `.bin`. The
// attachment service normalizes the name further, as for an upload.
func renderFileName(name, format, produces string) (string, error) {
	want := formatExtensions(format, produces)
	ext := strings.ToLower(filepath.Ext(name))
	if len(want) == 0 {
		// The host knows no extension for the format, so no name the script
		// picks can be checked against it. A neutral one keeps the download's
		// Content-Type from following a script-chosen extension.
		want = []string{".bin"}
	}
	// A suffix no type claims is part of the name, as in `audit-v1.2`.
	if ext == "" || (!slices.Contains(want, ext) && mime.TypeByExtension(ext) == "") {
		return name + want[0], nil
	}
	if slices.Contains(want, ext) {
		return name, nil
	}
	return "", fmt.Errorf("attach_render: filename %q does not match format %q (want %s)",
		name, format, strings.Join(want, " or "))
}

// formatExtensions lists the extensions of a format's output, first the
// preferred one.
func formatExtensions(format, produces string) []string {
	if format == markdownFormat && produces == markdownMIME {
		return []string{".md", ".markdown"}
	}
	exts, err := mime.ExtensionsByType(produces)
	if err != nil {
		return nil
	}
	return exts
}

// writeTarget resolves the entity and runs the upload preflight on it. The
// type is the stored one: the preflight then reads the row through the
// gates, so a hidden entity is refused exactly like an absent one.
//
// An absent id still runs the preflight, against a type that declares the
// property, so it answers the same message as a hidden id and costs at
// least as much (RR-NGMI, as in anchoredDocumentGates).
func (ar attachRenderer) writeTarget(
	ctx context.Context, s *Schema, addr, property string,
) (*entity.Entity, error) {
	notFound := describeWriteRefusal(addr, property, errWriteTargetNotFound)
	parsed, err := entity.ParseAddress(addr)
	if err != nil {
		return nil, notFound
	}
	typ := ar.a.visibleReader.storedType(ctx, parsed.ID())
	if typ == "" {
		typ = fallbackFileType(s, property)
	}
	if typ == "" {
		return nil, notFound
	}
	target, err := ar.a.attachments.attachmentWriteCheck(ctx, s, typ, addr, property)
	if err == nil {
		return target, nil
	}
	return nil, describeWriteRefusal(addr, property, err)
}

// fallbackFileType is the first type, by name, that declares property as a
// file property, or "" when none does.
func fallbackFileType(s *Schema, property string) string {
	if s == nil || s.Meta == nil {
		return ""
	}
	names := make([]string, 0, len(s.Meta.Entities))
	for name := range s.Meta.Entities {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if isFileProperty(s, name, property) {
			return name
		}
	}
	return ""
}

// describeWriteRefusal turns an [attachmentHandler.attachmentWriteCheck]
// error into the script's message.
func describeWriteRefusal(addr, property string, err error) error {
	var (
		amb    *visibility.AmbiguousAddressError
		fe     *acl.ForbiddenError
		denial AffordanceDenialError
	)
	switch {
	case errors.Is(err, errWriteTargetNotFound):
		// One message for a missing entity, a hidden one and a property it
		// does not have or the caller cannot see.
		return fmt.Errorf("attach_render: entity %q or its file property %q not found", addr, property)
	case errors.As(err, &amb):
		return fmt.Errorf("attach_render: %w", amb)
	case errors.Is(err, errEntityLocked):
		return fmt.Errorf("attach_render: entity %q is encrypted and cannot be edited", addr)
	case errors.As(err, &fe):
		return fmt.Errorf("attach_render: permission denied to update %q: %s", addr, fe.Decision.Reason)
	case errors.As(err, &denial):
		return fmt.Errorf("attach_render: property %q of %q cannot be changed: %s", property, addr, denial.Reason)
	default:
		return readCheckFailed(err)
	}
}

// readCheckFailed is the message for a gate that could not answer. The cause
// may name internals, so it goes to the log.
func readCheckFailed(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("attach_render: %w", err)
	}
	slog.Warn("attach_render: read check failed", "err", err)
	return errors.New("attach_render: the read check failed; see the server log")
}

// renderer runs the gates of the document or list and returns its markdown
// renderer, plus the source description for the audit record.
func (ar attachRenderer) renderer(
	ctx context.Context, s *Schema, req lua.AttachRenderRequest,
) (transform.Renderer, string, error) {
	if req.List != "" {
		return ar.listRenderer(ctx, s, req.List)
	}
	docCfg, ok := s.Cfg.Documents[req.Document]
	if !ok {
		return nil, "", fmt.Errorf("attach_render: unknown document %q", req.Document)
	}
	// An elevated document reads past the caller's ACL. Its output would
	// outlive the request as a file anyone who reads the entity can
	// download, so it is not attached.
	if docCfg.AllowACLBypass.Enabled() {
		return nil, "", fmt.Errorf("attach_render: document %q reads with elevated privileges and cannot be attached",
			req.Document)
	}
	var (
		resolved resolvedDocument
		err      error
	)
	if req.Entry == "" {
		resolved, err = standaloneDocumentGates(ctx, ar.a, s, req.Document)
	} else {
		resolved, err = anchoredDocumentGates(ctx, ar.a, s, req.Document, req.Entry)
	}
	if err != nil {
		return nil, "", describeGateDenial(err)
	}
	if len(resolved.cfg.Command) > 0 {
		return nil, "", fmt.Errorf("attach_render: document %q uses a command: renderer; only script: documents render",
			req.Document)
	}
	render := transform.RendererFunc(func(ctx context.Context) ([]byte, error) {
		md, err := ar.a.documents.RenderDocumentMarkdown(ctx, resolved.entryID, resolved.cfg)
		if err != nil {
			return nil, err
		}
		return []byte(md), nil
	})
	return render, fmt.Sprintf("document=%q entry=%q", req.Document, resolved.entryID), nil
}

// listRenderer is [attachRenderer.renderer] for a list: the query the list's
// own page sends, through the export pipeline.
func (ar attachRenderer) listRenderer(
	ctx context.Context, s *Schema, listID string,
) (transform.Renderer, string, error) {
	listCfg, ok := s.Cfg.Lists[listID]
	if !ok {
		return nil, "", fmt.Errorf("attach_render: unknown list %q", listID)
	}
	render, err := ar.a.export.listExportRenderer(ctx, listCfg.EntityType, navItemsListQuery(listID, listCfg))
	if err != nil {
		if errors.Is(err, errBadFilter) || errors.Is(err, errBadQueryScope) || errors.Is(err, errPropertyNotVisible) {
			return nil, "", fmt.Errorf("attach_render: list %q: %w", listID, err)
		}
		return nil, "", readCheckFailed(err)
	}
	return render, fmt.Sprintf("list=%q", listID), nil
}

// describeGateDenial is the script's message for a document gate's error.
func describeGateDenial(err error) error {
	var d *denialError
	if errors.As(err, &d) {
		return fmt.Errorf("attach_render: %s", d.Error())
	}
	return readCheckFailed(err)
}

// convert renders the markdown and converts it. A render or transform
// failure is logged; the script learns which step failed.
func (ar attachRenderer) convert(
	ctx context.Context, conv renderConversion, render transform.Renderer,
) (data []byte, mimeType string, err error) {
	if conv.name == "" {
		md, renderErr := render.Render(ctx)
		if renderErr != nil {
			return nil, "", conversionFailed("render", renderErr)
		}
		return md, conv.produces, nil
	}
	res, err := ar.a.export.engine.Run(ctx, conv.reg, conv.name, render)
	if err != nil {
		return nil, "", conversionFailed("transform "+conv.name, err)
	}
	return res.Data, res.Produces, nil
}

// conversionFailed is the message for a failed render or transform. Its
// cause can carry a command line or a script trace, so it goes to the log.
func conversionFailed(step string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("attach_render: %s: %w", step, err)
	}
	slog.Warn("attach_render: conversion failed", "step", step, "err", err)
	return fmt.Errorf("attach_render: %s failed; see the server log", step)
}

// store writes data like an upload: the configured size cap, the attachment
// policy and the entitymanager write. The caller holds the upload slot.
func (ar attachRenderer) store(
	ctx context.Context, s *Schema, target *entity.Entity, property, fileName string, data []byte,
) (*attachment.Result, error) {
	limit := maxAttachmentBytes(s)
	written, err := ar.a.attachments.storeUpload(ctx, s, target, property, fileName, bytes.NewReader(data), limit)
	if err == nil {
		return written, nil
	}
	var fe *acl.ForbiddenError
	switch {
	case isAttachmentTooLarge(err):
		return nil, fmt.Errorf("attach_render: the rendered file exceeds the %d byte attachment limit", limit)
	case errors.Is(err, attachment.ErrBusy):
		return nil, errors.New("attach_render: another write to this property is in progress; retry later")
	case errors.Is(err, attachment.ErrAtCapacity):
		return nil, fmt.Errorf("attach_render: property %q already holds the maximum number of files", property)
	case errors.Is(err, attachment.ErrRejected):
		return nil, fmt.Errorf("attach_render: attachment rejected: %s", attachment.RejectionReason(err))
	case errors.As(err, &fe):
		return nil, fmt.Errorf("attach_render: permission denied to update %q: %s", target.ID, fe.Decision.Reason)
	case errors.Is(err, store.ErrNotFound):
		// Deleted between the check and the write: the same message as any
		// other missing target.
		return nil, describeWriteRefusal(target.Ref().String(), property, errWriteTargetNotFound)
	default:
		slog.Warn("attach_render: write failed", "err", err)
		return nil, errors.New("attach_render: writing the attachment failed; see the server log")
	}
}
