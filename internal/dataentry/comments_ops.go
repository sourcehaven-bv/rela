package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The comment operations, separated from their HTTP routes (TKT-QIQO1W) so the
// remote MCP tools run the same gates. Each operation takes a ctx and returns
// an [opFailure] instead of writing a response; the route writes the
// failure with the writer it always used, and [MCPComments] renders it as a
// [comments.OpError].

// opFailure is a refused comment operation or action run (TKT-NW9QSN).
type opFailure struct {
	status int
	code   string
	title  string
	detail string
	// faces lists the addressable faces of a `face_required` failure.
	faces []string
	// cause is the underlying error: logged for an internal failure, never
	// shown to the caller.
	cause error
	// warn logs cause at Warn rather than Error, as writeGateError does.
	warn bool
	// correlationID matches a failed action run to its log line.
	correlationID string
	// write answers the failure on HTTP, for the failures this package has a
	// dedicated writer for. Nil means writeV1Error with the fields above.
	write func(w http.ResponseWriter, r *http.Request)
}

// writeHTTP answers f on the HTTP API.
func (f *opFailure) writeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.write != nil {
		f.write(w, r)
		return
	}
	writeV1Error(w, r, f.status, f.code, f.title, f.detail)
}

// opError describes f to a transport other than HTTP. A cancelled request
// returns the context error itself; an internal failure is logged here, as
// the HTTP writers log it.
func (f *opFailure) opError(ctx context.Context) error {
	if errors.Is(f.cause, context.Canceled) {
		return f.cause
	}
	f.logInternal(ctx, "comments: operation failed")
	return &comments.OpError{Code: f.code, Title: f.title, Detail: f.detail, Faces: f.faces}
}

// logInternal logs the cause of an internal failure, as the HTTP writers do.
func (f *opFailure) logInternal(ctx context.Context, msg string) {
	if f.status >= http.StatusInternalServerError && f.cause != nil {
		level := slog.LevelError
		if f.warn {
			level = slog.LevelWarn
		}
		slog.Log(ctx, level, msg, "code", f.code, "err", f.cause)
	}
}

func failV1(status int, code, title string) *opFailure {
	return &opFailure{status: status, code: code, title: title}
}

func failEntityNotFound() *opFailure {
	return failV1(http.StatusNotFound, "not_found", entityNotFoundTitle)
}

func failForbidden(title string) *opFailure {
	return failV1(http.StatusForbidden, "forbidden", title)
}

// failGate is the failure writeGateError answers.
func failGate(err error) *opFailure {
	// Built in two steps, here and in failInternal: TestNoInternalErrorDetail
	// requires every field of a literal naming a 500 to be a constant, and
	// the detail, the only field a caller sees, is.
	f := &opFailure{status: http.StatusInternalServerError, detail: internalErrorDetail}
	f.code, f.title, f.cause, f.warn = "acl_query_failed", "ACL read-permission check failed", err, true
	f.write = func(w http.ResponseWriter, r *http.Request) { writeGateError(w, r, err) }
	if errors.Is(err, context.DeadlineExceeded) {
		f.status, f.code, f.title = http.StatusGatewayTimeout, "acl_query_timeout",
			"ACL read-permission check timed out"
	}
	return f
}

// failWriteTarget is the failure writeWriteTargetError answers.
func failWriteTarget(err error) *opFailure {
	var amb *visibility.AmbiguousAddressError
	switch {
	case errors.As(err, &amb):
		faces := make([]string, len(amb.Faces))
		for i, face := range amb.Faces {
			faces[i] = entity.FormatStateRef(amb.ID, face)
		}
		return &opFailure{
			status: http.StatusUnprocessableEntity, code: "face_required", title: "Address one face",
			detail: amb.Error(), faces: faces,
			write: func(w http.ResponseWriter, r *http.Request) { writeFaceRequired(w, r, amb) },
		}
	case errors.Is(err, errWriteTargetNotFound):
		return failEntityNotFound()
	default:
		return failGate(err)
	}
}

// failInternal is the `comments_failed` failure writeInternalError answers.
func failInternal(title string, err error) *opFailure {
	const code = "comments_failed"
	f := &opFailure{status: http.StatusInternalServerError, detail: internalErrorDetail}
	f.code, f.title, f.cause = code, title, err
	f.write = func(w http.ResponseWriter, r *http.Request) { writeInternalError(w, r, code, title, err) }
	return f
}

// failACLDenied is the failure writeForbiddenIfACLDenied answers.
func failACLDenied(fe *acl.ForbiddenError) *opFailure {
	return &opFailure{
		status: http.StatusForbidden, code: "forbidden", title: "Denied by the access policy",
		detail: fmt.Sprintf("%s (rule_kind=%s rule_id=%s)", fe.Decision.Reason, fe.Decision.RuleKind, fe.Decision.RuleID),
		write:  func(w http.ResponseWriter, _ *http.Request) { writeForbiddenIfACLDenied(w, fe) },
	}
}

// failPatch is the failure writePatchError answers. Another transport sends
// no If-Match, so a version conflict is the plain `conflict`.
func failPatch(err error) *opFailure {
	write := func(w http.ResponseWriter, r *http.Request) { writePatchError(w, r, err) }
	var fe *acl.ForbiddenError
	if errors.As(err, &fe) {
		f := failACLDenied(fe)
		f.write = write
		return f
	}
	var conflict *store.VersionConflictError
	if errors.As(err, &conflict) {
		return &opFailure{
			status: http.StatusConflict, code: "conflict", title: "Entity is being modified concurrently",
			detail: "retry the request", write: write,
		}
	}
	return &opFailure{
		status: http.StatusUnprocessableEntity, code: "validation_failed", title: "Validation failed",
		detail: err.Error(), write: write,
	}
}

// failComment maps a comment service error, as writeCommentError did.
//
// Validation failures are 400s (malformed wire format, per DEC-HWZHA's first
// class) and everything unrecognized is a 500 with no detail — a storage error
// message can carry a path, and a path is not the caller's business.
func failComment(err error) *opFailure {
	switch {
	case errors.Is(err, comments.ErrNotFound):
		return failV1(http.StatusNotFound, "not_found", "Comment not found")
	case errors.Is(err, comments.ErrEmptyBody),
		errors.Is(err, comments.ErrBodyTooLong),
		errors.Is(err, comments.ErrBodyControlChars),
		errors.Is(err, comments.ErrInvalidAnchor),
		errors.Is(err, comments.ErrInvalidReplacement):
		return failV1(http.StatusBadRequest, "invalid_comment", err.Error())
	case errors.Is(err, comments.ErrTooManyComments):
		return failV1(http.StatusConflict, "too_many_comments", err.Error())
	case errors.Is(err, comments.ErrUnknownAuthor):
		// The caller is authenticated enough to reach here but has no
		// resolvable identity to attribute the comment to.
		return failForbidden("Comments require an identified author")
	default:
		return failInternal("Could not save the comment", err)
	}
}

// commentEnabled refuses a target type the metamodel does not enable
// comments for. Commentability is metamodel config, not a secret: an operator
// debugging a missing comment box needs to be told the type is not enabled.
func (h *commentsHandler) commentEnabled(typeName string) *opFailure {
	if !h.commentPolicy().Commentable(typeName) {
		return failV1(http.StatusBadRequest, "comments_not_enabled",
			"Comments are not enabled for this entity type")
	}
	return nil
}

// readOnlyFailure denies a comment write on a read-only instance.
//
// Checked before any other gate: ReadOnlyACL is a process-wide refusal an
// operator wires for "absolute confidence no writes happen", so it must not be
// satisfiable by any permission in any acl.yaml. Comments bypass the
// entitymanager, so ReadOnlyACL's blanket write deny never sees them — this is
// where that guarantee is kept for the commentary layer.
func (h *commentsHandler) readOnlyFailure() *opFailure {
	if h.commentWritesPermitted() {
		return nil
	}
	return failForbidden("this rela instance is configured read-only")
}

// readTarget resolves the target entity for a read of the thread. A bare id
// resolves to the face the request's world selects, as other reads do.
// Operations that write use [commentsHandler.writeTarget].
//
// Both "you may not read this" and "this does not exist" fail with the same
// 404. Checking EXISTENCE as well as the read verdict matters: the read gate
// answers "may this principal read this type", which a nonexistent id passes —
// so gating on the verdict alone would let a comment route confirm that an
// arbitrary id is absent, and (worse) accept comments filed against entities
// that were never there.
//
// The resolved entity is returned so the anchor code reads the SAME row the
// gate admitted. Re-reading it by id would lose the face: the thread is keyed
// by (id, face), and a bare-id read returns whichever face the world selects,
// which is how a draft comment came to be anchored in the published body.
func (h *commentsHandler) readTarget(
	ctx context.Context, addr commentAddress,
) (comments.Target, *entity.Entity, *opFailure) {
	target := comments.Target{Type: addr.typeName, ID: addr.ref.ID, Face: addr.ref.Face}
	// The resolver also checks the stored type against the route's, so a
	// type mismatch is the same miss as an absent id (ruling 9.2).
	ent, found, err := h.visibleReader.addressRef(ctx, addr.typeName, addr.ref)
	if err != nil {
		return target, nil, failGate(err)
	}
	if !found {
		return target, nil, failEntityNotFound()
	}
	// Scope the thread to the face this request actually resolved to
	// (FEAT-9CD2MX). Taken from the RESOLVED entity rather than the address:
	// a bare id under a world resolves to whichever face that world selects,
	// and the thread must match the content on screen.
	target.ID = ent.ID
	target.Face = ent.Face
	return target, ent, nil
}

// writeTarget is [commentsHandler.readTarget] for an operation that writes
// the thread, or prepares a write to it.
//
// A thread is stored per face, so a write must land on the face the caller
// names, never on the face a world ranks first. A named face is used as
// given, subject to the read gate. A bare id resolves through
// [visibility.Resolver.WriteTarget]: it names the implicit face of a faceless
// type, and on a faced type it is a 422 `face_required` listing the readable
// faces. Misses fail with the same uniform 404 as the read gate.
func (h *commentsHandler) writeTarget(
	ctx context.Context, addr commentAddress,
) (comments.Target, *entity.Entity, *opFailure) {
	ent, err := h.visibleReader.writeTarget(ctx, addr.typeName, addr.ref.String())
	if err != nil {
		return comments.Target{Type: addr.typeName}, nil, failWriteTarget(err)
	}
	return comments.Target{Type: addr.typeName, ID: ent.ID, Face: ent.Face}, ent, nil
}

// listedComment is one comment of a list, with its anchor resolved against the
// live entity.
type listedComment struct {
	view   comments.View
	anchor anchorWire
}

// listOp reads a target's thread as the ctx principal sees it.
func (h *commentsHandler) listOp(ctx context.Context, addr commentAddress) ([]listedComment, *opFailure) {
	auth := h.commentAuthorizer()

	// Read floor first: a caller who cannot read the target — or a target that
	// does not exist — gets the same 404, so comments never confirm existence.
	target, ent, fail := h.readTarget(ctx, addr)
	if fail != nil {
		return nil, fail
	}
	if !auth.CanRead(ctx, target) {
		return nil, failForbidden("Reading comments requires the comment:read permission")
	}

	list, err := h.svc.List(ctx, target)
	if err != nil {
		return nil, failInternal("Could not read comments", err)
	}

	user := principal.From(ctx).User
	anchors := h.liveAnchors(target, ent)
	out := make([]listedComment, 0, len(list))
	for _, c := range list {
		anchor, detached := resolveAnchor(anchors, c.Anchor)
		view := comments.NewView(c)
		view.Detached = detached
		view.Editable = auth.CanUpdate(ctx, target, c, user)
		view.Deletable = auth.CanDelete(ctx, target, c, user)
		view.Acceptable = !c.Resolved && anchors.loaded && anchors.body.Acceptable(c.Anchor)
		out = append(out, listedComment{view: view, anchor: anchor})
	}
	return out, nil
}

// addGate authorizes adding a comment, before the request body is read.
func (h *commentsHandler) addGate(
	ctx context.Context, addr commentAddress,
) (comments.Target, *entity.Entity, *opFailure) {
	if fail := h.readOnlyFailure(); fail != nil {
		return comments.Target{}, nil, fail
	}
	return h.addTargetGate(ctx, addr)
}

// addTargetGate is [commentsHandler.addGate] without the read-only refusal,
// for the resolve check, which writes nothing.
func (h *commentsHandler) addTargetGate(
	ctx context.Context, addr commentAddress,
) (comments.Target, *entity.Entity, *opFailure) {
	target, ent, fail := h.writeTarget(ctx, addr)
	if fail != nil {
		return target, nil, fail
	}
	if !h.commentAuthorizer().CanAdd(ctx, target) {
		return target, nil, failForbidden("Adding a comment requires the comment:add permission")
	}
	return target, ent, nil
}

// addOp adds a comment to a target [commentsHandler.addGate] admitted.
func (h *commentsHandler) addOp(
	ctx context.Context, target comments.Target, ent *entity.Entity, in comments.AddInput,
) (comments.Comment, *opFailure) {
	anchor := comments.Anchor{Kind: in.Kind, Ref: in.Ref, Replacement: in.Replacement}
	if anchor.Kind == comments.AnchorText {
		text, err := buildTextAnchor(ent, in.Quote, in.QuotePrefix, in.QuoteSuffix)
		if err != nil {
			return comments.Comment{}, failV1(http.StatusBadRequest, "invalid_comment", err.Error())
		}
		anchor.Text = text
	}
	created, err := h.svc.Add(ctx, target, comments.AddRequest{Anchor: anchor, Body: in.Body})
	if err != nil {
		return comments.Comment{}, failComment(err)
	}
	return created, nil
}

// mutationKind distinguishes the two mutating operations for mutationGate.
type mutationKind int

const (
	mutationUpdate mutationKind = iota
	mutationDelete
)

// mutationGate resolves the target, loads the comment, and authorizes the
// mutation — returning the loaded comment so the caller does not re-read it.
//
// The comment is loaded BEFORE the permission check because ownership is a
// property of the stored record: "may I edit this?" cannot be answered without
// knowing who wrote it. A missing comment is a 404 rather than a 403, since by
// this point the caller has proven it can read the target.
func (h *commentsHandler) mutationGate(
	ctx context.Context, addr commentAddress, commentID string, kind mutationKind,
) (comments.Target, comments.Comment, *opFailure) {
	if fail := h.readOnlyFailure(); fail != nil {
		return comments.Target{}, comments.Comment{}, fail
	}
	target, _, fail := h.writeTarget(ctx, addr)
	if fail != nil {
		return target, comments.Comment{}, fail
	}

	existing, err := h.svc.Get(ctx, target, commentID)
	if err != nil {
		if errors.Is(err, comments.ErrNotFound) {
			return target, comments.Comment{}, failV1(http.StatusNotFound, "not_found", "Comment not found")
		}
		return target, comments.Comment{}, failInternal("Could not read comments", err)
	}

	user := principal.From(ctx).User
	auth := h.commentAuthorizer()
	allowed := auth.CanUpdate(ctx, target, existing, user)
	perms := "comment:update-own or comment:update-any"
	if kind == mutationDelete {
		allowed = auth.CanDelete(ctx, target, existing, user)
		perms = "comment:delete-own or comment:delete-any"
	}
	if !allowed {
		return target, comments.Comment{}, failForbidden("This action requires " + perms)
	}
	return target, existing, nil
}

// updateOp edits a comment [commentsHandler.mutationGate] admitted. A nil
// field keeps its stored value, so resolving a comment does not require
// echoing its body back.
func (h *commentsHandler) updateOp(
	ctx context.Context, target comments.Target, existing comments.Comment, commentID string,
	body *string, resolved *bool,
) *opFailure {
	newBody := existing.Body
	if body != nil {
		newBody = *body
	}
	newResolved := existing.Resolved
	if resolved != nil {
		newResolved = *resolved
	}
	if err := h.svc.Update(ctx, target, commentID, newBody, newResolved); err != nil {
		return failComment(err)
	}
	return nil
}

// deleteOp removes a comment.
func (h *commentsHandler) deleteOp(ctx context.Context, addr commentAddress, commentID string) *opFailure {
	target, _, fail := h.mutationGate(ctx, addr, commentID, mutationDelete)
	if fail != nil {
		return fail
	}
	if err := h.svc.Delete(ctx, target, commentID); err != nil {
		return failComment(err)
	}
	return nil
}

// authorizeBodyWrite pre-checks the `update` write an accept will perform,
// recording a denial the way the attachment handler does.
func (h *commentsHandler) authorizeBodyWrite(ctx context.Context, e *entity.Entity) *opFailure {
	decision := h.acl().AuthorizeWrite(ctx, translateVerb("update", e.Type, e.ID, e.Face))
	if decision.Allow {
		return nil
	}
	h.audit().Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     &audit.Subject{Kind: "entity", Type: e.Type, ID: e.ID},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary: fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s op=accept-suggestion)",
			decision.Reason, decision.RuleKind, decision.RuleID),
	})
	return failACLDenied(&acl.ForbiddenError{Decision: decision})
}

// openSuggestion loads the comment an accept applies. It fails unless the
// comment exists, is readable, suggests a replacement and is still open.
func (h *commentsHandler) openSuggestion(
	ctx context.Context, target comments.Target, commentID string,
) (comments.Comment, *opFailure) {
	// Before the lookup, so a principal without comment:read cannot tell an
	// existing comment id (403) from an unknown one (404).
	if !h.commentAuthorizer().CanRead(ctx, target) {
		return comments.Comment{}, failForbidden("Accepting a suggestion requires the comment:read permission")
	}
	existing, err := h.svc.Get(ctx, target, commentID)
	if err != nil {
		return comments.Comment{}, failComment(err)
	}
	if existing.Anchor.Replacement == nil {
		return comments.Comment{}, failV1(http.StatusConflict, "no_suggestion",
			"This comment does not suggest a replacement")
	}
	if existing.Resolved {
		return comments.Comment{}, failV1(http.StatusConflict, "comment_resolved",
			"This comment is already resolved")
	}
	return existing, nil
}

// acceptOp applies a comment's suggested replacement to the entity body and
// resolves the comment (TKT-S5C0K3). ctx must already be provisioned and the
// read-only refusal checked; the route and [MCPComments] both do so first.
//
// # Authorization
//
// Two independent grants, both required. `comment:read` (with the target read
// floor) covers seeing the suggestion. The entity write is authorized by
// PatchEntity itself, exactly as any other body edit, so a principal who could
// not have typed this change cannot accept it either. That write is also
// pre-checked, so a certain denial never touches the comment. `comment:update-*` is
// NOT required to resolve the comment here: the accepter is usually not its
// author, and closing a suggestion by applying it is part of the edit.
//
// The person accepting is the author of record for the body change (audit log,
// version history), in the way whoever merges a pull request is responsible
// for it.
//
// # Order: claim, then write
//
// The comment and the entity live in different stores, so this cannot be
// atomic. Resolving FIRST makes the partial failure "resolved but not applied",
// which the user can see and undo by reopening. The other order fails as
// "applied but still open", and a replacement that keeps the quoted text
// ("brown fox" → "brown fox jumps") still matches, so a retry would apply it
// twice.
//
// The resolve is a conditional [comments.Service.SetResolved], not a write of
// the whole comment. The flip is what stops two requests, on one server
// process or several sharing a database, from both applying the suggestion,
// and it leaves a concurrent edit to the comment's body intact. The patch
// carries the raw row's version, so a body write landing after the reads
// fails the patch instead of being overwritten.
func (h *commentsHandler) acceptOp(
	ctx context.Context, addr commentAddress, commentID string,
) (comments.Accepted, *opFailure) {
	target, visible, fail := h.writeTarget(ctx, addr)
	if fail != nil {
		return comments.Accepted{}, fail
	}
	existing, fail := h.openSuggestion(ctx, target, commentID)
	if fail != nil {
		return comments.Accepted{}, fail
	}

	// The splice base and version token come from the RAW row at the face the
	// gate resolved. The visible entity may have redacted properties, and a
	// version token hashed over redacted properties never matches the store.
	raw, found := h.reader.writePrepRow(ctx, entity.Ref{ID: target.ID, Face: target.Face})
	if !found {
		return comments.Accepted{}, failEntityNotFound()
	}
	if raw.IsLocked() {
		f := failV1(http.StatusUnprocessableEntity, "encrypted_inaccessible", "Cannot edit an inaccessible entity")
		f.detail = "File is git-crypt encrypted; run `git-crypt unlock` first."
		return comments.Accepted{}, f
	}
	// The manager stays the authority on the write, but a caller it is certain
	// to refuse must not get to resolve and reopen someone else's comment on
	// the way to that refusal.
	if fail := h.authorizeBodyWrite(ctx, raw); fail != nil {
		return comments.Accepted{}, fail
	}
	// The accepter reviewed the suggestion against the body they can see. If
	// that differs from the stored body, writing would splice into text they
	// never saw, so refuse. Body redaction does not exist today, so in practice
	// this is a concurrent write landing between the two reads; the message
	// says so.
	if raw.Content != visible.Content {
		return comments.Accepted{}, failV1(http.StatusConflict, "suggestion_stale",
			"The entity body changed while accepting; reload and try again")
	}

	newBody, err := comments.ApplyReplacement(raw.Content, existing.Anchor)
	if err != nil {
		return comments.Accepted{}, failV1(http.StatusConflict, "suggestion_stale",
			"The suggested text no longer matches the entity body")
	}

	// Claim the comment. The flip is conditional in the store, so of two
	// accepts racing (even on two server processes) exactly one proceeds.
	claimed, err := h.svc.SetResolved(ctx, target, commentID, true)
	if err != nil {
		return comments.Accepted{}, failComment(err)
	}
	if !claimed {
		return comments.Accepted{}, failV1(http.StatusConflict, "comment_resolved",
			"This comment is already resolved")
	}

	result, err := h.patcher.PatchEntity(ctx, target.Key(), entity.Patch{
		Content:         &newBody,
		ExpectedVersion: string(store.VersionOf(raw)),
	})
	if err != nil {
		if _, uerr := h.svc.SetResolved(ctx, target, commentID, false); uerr != nil {
			slog.ErrorContext(ctx, "accept suggestion: reopen after failed write",
				"target", target.Key(), "comment", commentID, "write_error", err, "reopen_error", uerr)
			return comments.Accepted{}, failV1(http.StatusInternalServerError, "comments_failed",
				"The suggestion was not applied, but the comment stayed resolved; reopen it to try again")
		}
		if errors.Is(err, entitymanager.ErrEntityNotFound) || errors.Is(err, store.ErrNotFound) {
			return comments.Accepted{}, failEntityNotFound()
		}
		return comments.Accepted{}, failPatch(err)
	}

	out := comments.Accepted{Content: newBody}
	if result != nil {
		if result.Entity != nil {
			out.Content = result.Entity.Content
		}
		out.Warnings = result.Warnings
	}
	return out, nil
}
