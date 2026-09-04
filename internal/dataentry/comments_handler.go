package dataentry

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// commentsPathPrefix is the reserved route segment for the commentary layer.
const commentsPathPrefix = "/api/v1/_comments/"

// handleV1Comments serves the commentary layer.
//
//	GET    /api/v1/_comments/{type}/{id}              → list a target's comments
//	POST   /api/v1/_comments/{type}/{id}              → add one
//	PATCH  /api/v1/_comments/{type}/{id}/{commentID}  → edit body / resolve
//	DELETE /api/v1/_comments/{type}/{id}/{commentID}  → remove one
//
// # Gating
//
// Every request resolves TWO things before it touches storage: the target's own
// read verdict, and the relevant `comment:*` permission for that target. The
// read verdict is the floor — a principal who cannot read an entity cannot
// learn anything about its comments, however the comment grants read, because
// otherwise a thread becomes an existence oracle for entities the principal is
// denied.
//
// A denial is reported as an indistinguishable 404 when it would otherwise
// confirm the target exists, and as a 403 naming the missing permission when
// the caller has already proven it can read the target. That split follows the
// repo's rule that entity existence is secret but a config-declared capability
// is not: telling an authorized reader which permission it lacks is the answer
// that helps the operator debug.
//
// # Disabled is absent, not forbidden
//
// With no `comments:` block the service is nil and every route 404s, so a
// project that never enables commenting is indistinguishable from one built
// before the feature existed.
func (h *commentsHandler) handleV1Comments(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		// Not a capability gap worth explaining (unlike version history, which
		// is backend-dependent): commenting is off because this project did
		// not ask for it, so the route simply does not exist.
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Not found", "")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, commentsPathPrefix)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_comments/{type}/{id}[/{commentID}]", "")
		return
	}
	typeName, entityID := parts[0], parts[1]

	// Validate the path segments before they reach storage. The file backend
	// guards itself too, but refusing here keeps an unsafe id from reaching any
	// backend at all, and gives a 400 rather than an opaque store error.
	if !isSafePathSegment(typeName) || !isSafePathSegment(entityID) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Invalid entity type or id", "")
		return
	}

	// Commentability is metamodel config, not a secret: an operator debugging
	// a missing comment box needs to be told the type is not enabled.
	if !h.commentPolicy().Commentable(typeName) {
		writeV1Error(w, r, http.StatusBadRequest, "comments_not_enabled",
			"Comments are not enabled for this entity type", "")
		return
	}

	target := comments.Target{Type: typeName, ID: entityID}

	switch {
	case len(parts) == 2:
		h.commentCollection(w, r, target)
	case len(parts) == 3 && parts[2] != "":
		h.commentItem(w, r, target, parts[2])
	default:
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_comments/{type}/{id}[/{commentID}]", "")
	}
}

// commentCollection handles the target-level routes.
func (h *commentsHandler) commentCollection(
	w http.ResponseWriter, r *http.Request, target comments.Target,
) {
	switch r.Method {
	case http.MethodGet:
		h.listComments(w, r, target)
	case http.MethodPost:
		if !h.refuseIfReadOnly(w, r) {
			h.addComment(w, r, target)
		}
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// commentItem handles the single-comment routes.
func (h *commentsHandler) commentItem(
	w http.ResponseWriter, r *http.Request, target comments.Target, commentID string,
) {
	switch r.Method {
	case http.MethodPatch:
		if !h.refuseIfReadOnly(w, r) {
			h.updateComment(w, r, target, commentID)
		}
	case http.MethodDelete:
		if !h.refuseIfReadOnly(w, r) {
			h.deleteComment(w, r, target, commentID)
		}
	case http.MethodOptions:
		w.Header().Set("Allow", "PATCH, DELETE, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "PATCH, DELETE, OPTIONS")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// commentListResponse is the list wire shape.
//
// An object rather than a bare array so the response can gain fields (a
// truncation flag, an unresolved count) without a breaking change.
type commentListResponse struct {
	Comments []commentWire `json:"comments"`
}

// commentWire is one comment on the wire.
//
// Deliberately NOT the storage type: `detached` is computed per request from
// the live entity, and re-using the stored struct would eventually tempt
// someone to persist it.
type commentWire struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	Anchor    struct {
		Kind string `json:"kind"`
		Ref  string `json:"ref"`
	} `json:"anchor"`
	Body     string `json:"body"`
	Resolved bool   `json:"resolved"`
	// Detached reports that the anchor no longer names anything on the target.
	// A soft condition per DEC-HWZHA: the comment is still returned and still
	// readable, flagged so the UI can show it as orphaned rather than pretend
	// it points somewhere.
	Detached bool `json:"detached,omitempty"`
	// Editable and Deletable are UI hints mirroring `_actions`: the server
	// re-authorizes every write, so a client that ignores them gains nothing.
	Editable  bool `json:"editable"`
	Deletable bool `json:"deletable"`
}

func (h *commentsHandler) listComments(
	w http.ResponseWriter, r *http.Request, target comments.Target,
) {
	ctx := r.Context()
	auth := h.commentAuthorizer()

	// Read floor first: a caller who cannot read the target — or a target that
	// does not exist — gets the same 404, so comments never confirm existence.
	if !h.gateCommentTarget(w, r, target) {
		return
	}
	if !auth.CanRead(ctx, target) {
		writeV1Error(w, r, http.StatusForbidden, "forbidden",
			"Reading comments requires the comment:read permission", "")
		return
	}

	list, err := h.svc.List(ctx, target)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "comments_failed",
			"Could not read comments", "")
		return
	}

	user := principal.From(ctx).User
	anchors := h.liveAnchors(ctx, target)
	out := make([]commentWire, 0, len(list))
	for _, c := range list {
		out = append(out, commentWire{
			ID:        c.ID,
			Author:    c.Author,
			CreatedAt: c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Anchor: struct {
				Kind string `json:"kind"`
				Ref  string `json:"ref"`
			}{Kind: string(c.Anchor.Kind), Ref: c.Anchor.Ref},
			Body:      c.Body,
			Resolved:  c.Resolved,
			Detached:  isDetached(anchors, c.Anchor),
			Editable:  auth.CanUpdate(ctx, target, c, user),
			Deletable: auth.CanDelete(ctx, target, c, user),
		})
	}

	writeV1JSON(w, http.StatusOK, commentListResponse{Comments: out})
}

// addCommentRequest is the create wire shape.
//
// It carries no author, id or timestamp: those are server-written, so the wire
// type cannot express them rather than accepting and discarding them.
type addCommentRequest struct {
	Anchor struct {
		Kind string `json:"kind"`
		Ref  string `json:"ref"`
	} `json:"anchor"`
	Body string `json:"body"`
}

func (h *commentsHandler) addComment(
	w http.ResponseWriter, r *http.Request, target comments.Target,
) {
	ctx := r.Context()

	if !h.gateCommentTarget(w, r, target) {
		return
	}
	if !h.commentAuthorizer().CanAdd(ctx, target) {
		writeV1Error(w, r, http.StatusForbidden, "forbidden",
			"Adding a comment requires the comment:add permission", "")
		return
	}

	var req addCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", "")
		return
	}

	created, err := h.svc.Add(ctx, target, comments.AddRequest{
		Anchor: comments.Anchor{
			Kind: comments.AnchorKind(req.Anchor.Kind),
			Ref:  req.Anchor.Ref,
		},
		Body: req.Body,
	})
	if err != nil {
		writeCommentError(w, r, err)
		return
	}

	writeV1JSON(w, http.StatusCreated, commentWire{
		ID:        created.ID,
		Author:    created.Author,
		CreatedAt: created.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Anchor: struct {
			Kind string `json:"kind"`
			Ref  string `json:"ref"`
		}{Kind: string(created.Anchor.Kind), Ref: created.Anchor.Ref},
		Body:      created.Body,
		Resolved:  created.Resolved,
		Editable:  true,
		Deletable: true,
	})
}

// updateCommentRequest carries the two mutable fields.
//
// Pointers so "not supplied" is distinguishable from "set to the zero value" —
// resolving a comment must not require echoing its body back.
type updateCommentRequest struct {
	Body     *string `json:"body"`
	Resolved *bool   `json:"resolved"`
}

func (h *commentsHandler) updateComment(
	w http.ResponseWriter, r *http.Request, target comments.Target, commentID string,
) {
	ctx := r.Context()

	existing, ok := h.gateCommentMutation(w, r, target, commentID, mutationUpdate)
	if !ok {
		return
	}

	var req updateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", "")
		return
	}

	body := existing.Body
	if req.Body != nil {
		body = *req.Body
	}
	resolved := existing.Resolved
	if req.Resolved != nil {
		resolved = *req.Resolved
	}

	if err := h.svc.Update(ctx, target, commentID, body, resolved); err != nil {
		writeCommentError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *commentsHandler) deleteComment(
	w http.ResponseWriter, r *http.Request, target comments.Target, commentID string,
) {
	if _, ok := h.gateCommentMutation(w, r, target, commentID, mutationDelete); !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), target, commentID); err != nil {
		writeCommentError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mutationKind distinguishes the two mutating routes for gateCommentMutation.
type mutationKind int

const (
	mutationUpdate mutationKind = iota
	mutationDelete
)

// gateCommentMutation resolves the target, loads the comment, and authorizes
// the mutation — returning the loaded comment so the caller does not re-read it.
//
// The comment is loaded BEFORE the permission check because ownership is a
// property of the stored record: "may I edit this?" cannot be answered without
// knowing who wrote it. A missing comment is a 404 rather than a 403, since by
// this point the caller has proven it can read the target.
func (h *commentsHandler) gateCommentMutation(
	w http.ResponseWriter, r *http.Request,
	target comments.Target, commentID string, kind mutationKind,
) (comments.Comment, bool) {
	ctx := r.Context()

	if !h.gateCommentTarget(w, r, target) {
		return comments.Comment{}, false
	}

	existing, err := h.svc.Get(ctx, target, commentID)
	if err != nil {
		if errors.Is(err, comments.ErrNotFound) {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Comment not found", "")
			return comments.Comment{}, false
		}
		writeV1Error(w, r, http.StatusInternalServerError, "comments_failed",
			"Could not read comments", "")
		return comments.Comment{}, false
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
		writeV1Error(w, r, http.StatusForbidden, "forbidden",
			"This action requires "+perms, "")
		return comments.Comment{}, false
	}
	return existing, true
}

// gateCommentTarget resolves the target entity, reporting whether the request
// may proceed.
//
// Both "you may not read this" and "this does not exist" answer with the same
// 404. Checking EXISTENCE as well as the read verdict matters: the read gate
// answers "may this principal read this type", which a nonexistent id passes —
// so gating on the verdict alone would let a comment route confirm that an
// arbitrary id is absent, and (worse) accept comments filed against entities
// that were never there.
func (h *commentsHandler) gateCommentTarget(w http.ResponseWriter, r *http.Request, target comments.Target) bool {
	_, found, err := h.visibleReader.getVisible(r.Context(), target.Type, target.ID)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return false
	}
	return true
}

// refuseIfReadOnly denies a comment write on a read-only instance, reporting
// whether it did so.
//
// Checked before any other gate: ReadOnlyACL is a process-wide refusal an
// operator wires for "absolute confidence no writes happen", so it must not be
// satisfiable by any permission in any acl.yaml. Comments bypass the
// entitymanager, so ReadOnlyACL's blanket write deny never sees them — this is
// where that guarantee is kept for the commentary layer.
func (h *commentsHandler) refuseIfReadOnly(w http.ResponseWriter, r *http.Request) bool {
	if h.commentWritesPermitted() {
		return false
	}
	writeV1Error(w, r, http.StatusForbidden, "forbidden",
		"this rela instance is configured read-only", "")
	return true
}

// isDetached reports whether a comment's anchor no longer names anything.
//
// Only property anchors are checked. A section ref resolves against the view
// config rather than the entity, so this per-entity view cannot tell a missing
// section from a present one — and guessing would put a "detached" badge on
// every section comment. A nil set means the entity could not be loaded, which
// likewise proves nothing.
func isDetached(liveProperties map[string]bool, anchor comments.Anchor) bool {
	if liveProperties == nil || anchor.Kind != comments.AnchorProperty {
		return false
	}
	return !liveProperties[anchor.Ref]
}

// writeCommentError maps a service error to a response.
//
// Validation failures are 400s (malformed wire format, per DEC-HWZHA's first
// class) and everything unrecognized is a 500 with no detail — a storage error
// message can carry a path, and a path is not the caller's business.
func writeCommentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, comments.ErrNotFound):
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Comment not found", "")
	case errors.Is(err, comments.ErrEmptyBody),
		errors.Is(err, comments.ErrBodyTooLong),
		errors.Is(err, comments.ErrBodyControlChars),
		errors.Is(err, comments.ErrInvalidAnchor):
		writeV1Error(w, r, http.StatusBadRequest, "invalid_comment", err.Error(), "")
	case errors.Is(err, comments.ErrTooManyComments):
		writeV1Error(w, r, http.StatusConflict, "too_many_comments", err.Error(), "")
	case errors.Is(err, comments.ErrUnknownAuthor):
		// The caller is authenticated enough to reach here but has no
		// resolvable identity to attribute the comment to.
		writeV1Error(w, r, http.StatusForbidden, "forbidden",
			"Comments require an identified author", "")
	default:
		writeV1Error(w, r, http.StatusInternalServerError, "comments_failed",
			"Could not save the comment", "")
	}
}
