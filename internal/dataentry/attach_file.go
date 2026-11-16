package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// opAttachFile names rela.attach_file in its messages and log lines.
const opAttachFile = "attach_file"

// attachFiler is rela.attach_file (TKT-36DHKH8): it stores bytes the script
// holds on a file property, as the ctx principal, like an upload of the same
// bytes.
//
// It runs the chain [attachRenderer] runs, minus the render: the request
// state a script context lacks ([bindScriptRequestState]), the upload
// preflight ([scriptWriteTarget]), the upload slot and the upload write
// ([storeScriptFile]). The entitymanager records the property write, as for
// an upload.
//
// A type of its own rather than App methods: App is at its plimsoll method
// cap.
type attachFiler struct {
	a *App
}

// newAttachFiler returns the filer over a.
//
// Nil: a nil App, or one without its attachment handler, is rejected.
func newAttachFiler(a *App) (attachFiler, error) {
	switch {
	case a == nil:
		return attachFiler{}, errors.New("dataentry: newAttachFiler: app is required")
	case a.attachments == nil || a.attachments.uploads == nil:
		return attachFiler{}, errors.New("dataentry: newAttachFiler: attachment handler is required")
	}
	return attachFiler{a: a}, nil
}

// AttachFiler returns rela.attach_file over a, for a host that runs scripts
// outside the data-entry app's own actions (schedules, flow steps and
// background automations, see appbuild.SetAttachFiler).
func AttachFiler(a *App) (lua.AttachFiler, error) {
	return newAttachFiler(a)
}

// actionAttachFiler is the filer an action script gets, or nil when a
// collaborator is missing, which leaves the binding absent. The missing
// collaborator is logged: it is a wiring fault, not a script error.
func actionAttachFiler(a *App) lua.AttachFiler {
	af, err := newAttachFiler(a)
	if err != nil {
		slog.Warn("attach_file unavailable to actions", "err", err)
		return nil
	}
	return af
}

// AttachFile implements [lua.AttachFiler]. Errors are written for the script
// author, as for [attachRenderer.AttachRender].
func (af attachFiler) AttachFile(ctx context.Context, req lua.AttachFileRequest) (lua.AttachedFile, error) {
	ctx, err := bindScriptRequestState(ctx, af.a, opAttachFile)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	s := af.a.State()
	// Refused before the preflight and the upload slot, as the upload route
	// refuses a body whose declared length is over the cap.
	if limit := maxAttachmentBytes(s); int64(len(req.Content)) > limit {
		return lua.AttachedFile{}, fmt.Errorf("attach_file: the file exceeds the %d byte attachment limit", limit)
	}

	fileName, err := checkDeclaredMIME(req)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	target, err := scriptWriteTarget(ctx, af.a, s, opAttachFile, req.Entity, req.Property)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	// The bytes are already in memory, but the slot still bounds how many
	// attachment writes (and scans) run at once, as for an HTTP upload.
	release, admitted := af.a.attachments.uploads.TryAcquire()
	if !admitted {
		return lua.AttachedFile{}, errors.New("attach_file: too many uploads in progress; retry later")
	}
	defer release()

	written, err := storeScriptFile(ctx, af.a, s, opAttachFile, target, req.Property, fileName, req.Content)
	if err != nil {
		return lua.AttachedFile{}, err
	}
	// The type a download of the file serves, without its parameters, like
	// the type rela.attach_render reports.
	served, _, err := mime.ParseMediaType(contentTypeForFilename(written.FileName))
	if err != nil {
		served = "application/octet-stream"
	}
	return lua.AttachedFile{
		Name: written.FileName, Size: written.Size, SHA256: written.SHA256, MIME: served,
	}, nil
}

// checkDeclaredMIME checks the bytes and the file name against the type the
// script declared, and returns the name to store. With no declared type the
// name is used as given, as for an upload.
//
// The bytes must not contradict the declared type, judged by the sniffing
// the attachment allowlist applies ([sniffContradicts]), so an error page
// fetched in place of a PDF is refused rather than stored as one. The name
// gets the type's extension when it has none, and is refused when its
// extension belongs to another type, as for rela.attach_render.
func checkDeclaredMIME(req lua.AttachFileRequest) (string, error) {
	if req.MIME == "" {
		return req.Filename, nil
	}
	declared, _, err := mime.ParseMediaType(req.MIME)
	if err != nil {
		return "", fmt.Errorf("attach_file: mime %q is not a media type", req.MIME)
	}
	if sniffed := attachment.SniffMIME(req.Content); sniffContradicts(sniffed, declared) {
		return "", fmt.Errorf("attach_file: content is %s, not the declared %s", sniffed, declared)
	}
	want := mimeExtensions(declared)
	name, ok := fileNameWithExtension(req.Filename, want)
	if !ok {
		return "", fmt.Errorf("attach_file: filename %q does not match mime %q (want %s)",
			req.Filename, declared, strings.Join(want, " or "))
	}
	return name, nil
}

// fingerprintedTypes are types [attachment.SniffMIME] recognizes by their
// bytes. Bytes declared as one of them must sniff as it.
var fingerprintedTypes = map[string]bool{
	"application/pdf":    true,
	"application/zip":    true,
	"application/x-gzip": true,
	"image/png":          true,
	"image/jpeg":         true,
	"image/gif":          true,
	"image/webp":         true,
	"image/bmp":          true,
	"text/html":          true,
}

// genericSniffs are the results sniffing gives when it recognizes nothing
// more specific: JSON and CSV sniff as text/plain, an Office document as
// application/zip, most other binaries as application/octet-stream.
var genericSniffs = map[string]bool{
	"text/plain":               true,
	"application/zip":          true,
	"application/octet-stream": true,
}

// sniffContradicts reports whether bytes that sniffed as sniffed cannot be
// of the declared type: the declared type is one sniffing fingerprints and
// the bytes are not it, or sniffing found a specific type other than the
// declared one.
func sniffContradicts(sniffed, declared string) bool {
	switch {
	case sniffed == declared:
		return false
	case fingerprintedTypes[declared]:
		return true
	default:
		return !genericSniffs[sniffed]
	}
}
