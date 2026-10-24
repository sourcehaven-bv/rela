package dataentry

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// commentsPathPrefix is the reserved route segment for the commentary layer.
const commentsPathPrefix = "/api/v1/_comments/"

// handleV1Comments serves the commentary layer.
//
//	GET    /api/v1/_comments/{type}/{id}              → list a target's comments
//	POST   /api/v1/_comments/{type}/{id}              → add one
//	PATCH  /api/v1/_comments/{type}/{id}/{commentID}  → edit body / resolve
//	DELETE /api/v1/_comments/{type}/{id}/{commentID}  → remove one
//	POST   /api/v1/_comments/{type}/{id}/{commentID}/accept → apply its suggestion
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
//
//archplan:auth=authz
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
	//
	// The id segment is an ADDRESS (`ID` or `ID@face`) and is parsed here,
	// once, into the bare id and the face (BUG-R1PQY9). Handing the raw
	// segment to a bare-id reader resolves on memstore/fsstore only because
	// their index key is the same string, and matches nothing on the database
	// backends or under a query-shaped read grant.
	addr, ok := parseCommentAddress(typeName, entityID)
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Invalid entity type or id", "")
		return
	}

	if fail := h.commentEnabled(typeName); fail != nil {
		fail.writeHTTP(w, r)
		return
	}

	switch {
	case len(parts) == 3 && parts[2] == "resolve":
		h.commentResolveCheck(w, r, addr)
	case len(parts) == 2:
		h.commentCollection(w, r, addr)
	case len(parts) == 3 && parts[2] != "":
		h.commentItem(w, r, addr, parts[2])
	case len(parts) == 4 && parts[2] != "" && parts[3] == "accept":
		h.commentAccept(w, r, addr, parts[2])
	default:
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_comments/{type}/{id}[/{commentID}]", "")
	}
}

// commentCollection handles the target-level routes.
func (h *commentsHandler) commentCollection(
	w http.ResponseWriter, r *http.Request, addr commentAddress,
) {
	switch r.Method {
	case http.MethodGet:
		h.listComments(w, r, addr)
	case http.MethodPost:
		h.addComment(w, r, addr)
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
	w http.ResponseWriter, r *http.Request, addr commentAddress, commentID string,
) {
	switch r.Method {
	case http.MethodPatch:
		h.updateComment(w, r, addr, commentID)
	case http.MethodDelete:
		h.deleteComment(w, r, addr, commentID)
	case http.MethodOptions:
		w.Header().Set("Allow", "PATCH, DELETE, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "PATCH, DELETE, OPTIONS")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// resolveCheckRequest asks whether a selection could be anchored.
type resolveCheckRequest struct {
	Quote  string `json:"quote"`
	Prefix string `json:"quote_prefix"`
	Suffix string `json:"quote_suffix"`
}

// resolveCheckResponse answers it, with a reason when it could not.
type resolveCheckResponse struct {
	Anchorable bool   `json:"anchorable"`
	Reason     string `json:"reason,omitempty"`
	// SourceQuote is the markdown SOURCE the selection maps to, which is what
	// a suggested replacement replaces. A client pre-filling a suggestion must
	// start from this, not from the rendered selection: the two differ
	// wherever markup sits inside the range, and editing the rendered form
	// would drop that markup on accept. It is a substring of a body the caller
	// may already read.
	SourceQuote string `json:"source_quote,omitempty"`
	// Suggestable reports whether a replacement for this selection would be
	// accepted, so a client can hide the option instead of letting someone
	// type a suggestion that is refused on submit.
	Suggestable bool `json:"suggestable,omitempty"`
}

// commentResolveCheck reports whether a selection can be anchored, WITHOUT
// creating anything.
//
// Exists because the alternative is worse: a user selects text, writes a
// comment, and only then learns it cannot be saved. Some selections genuinely
// cannot anchor — one spanning table cells has no contiguous source range —
// and the UI needs to know before it offers the affordance.
//
// Gated exactly like a create: it reveals whether a string appears in the body,
// which is a read of the body, so `comment:add` plus the target read verdict.
func (h *commentsHandler) commentResolveCheck(
	w http.ResponseWriter, r *http.Request, addr commentAddress,
) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	_, ent, fail := h.addTargetGate(r.Context(), addr)
	if fail != nil {
		fail.writeHTTP(w, r)
		return
	}

	var req resolveCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", "")
		return
	}

	text, err := buildTextAnchor(ent, req.Quote, req.Prefix, req.Suffix)
	if err != nil {
		// A failure here is the ANSWER, not an error: the caller asked whether
		// this selection works, and "no, because…" is a successful reply.
		writeV1JSON(w, http.StatusOK, resolveCheckResponse{Reason: err.Error()})
		return
	}
	writeV1JSON(w, http.StatusOK, resolveCheckResponse{
		Anchorable: true, SourceQuote: text.Quote, Suggestable: comments.Replaceable(text),
	})
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
// anchorWire is a comment's anchor on the wire.
//
// Named rather than inlined because it is built at three sites and carries the
// stage-2 text descriptors; an anonymous struct repeated four times drifts.
type anchorWire struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
	// Quote is the anchored text, echoed for a text anchor so a client can
	// show WHAT was commented on even when the range no longer resolves.
	Quote string `json:"quote,omitempty"`
	// Start and End are byte offsets into the entity body, resolved fresh on
	// every read. Present only for a text anchor that located successfully.
	//
	// They are NOT stored: an offset is invalidated by any edit earlier in the
	// body, which is the entire reason the descriptors exist. A client must
	// slice with these, never with the quote's length.
	Start *int `json:"start,omitempty"`
	End   *int `json:"end,omitempty"`
	// Segments splits [Start, End) into one byte range per markdown block, so
	// a client can highlight a range that crosses blocks with one element per
	// block: a single inline element cannot cross a block boundary. Present
	// whenever Start/End are. An EMPTY list means nothing in the range can be
	// highlighted (it is all code); a client must not fall back to Start/End
	// then, since marking inside code renders the markup literally.
	Segments *[]spanWire `json:"segments,omitempty"`
	// Confidence is the resolver's score for a text anchor (0-1).
	Confidence float64 `json:"confidence,omitempty"`
	// Uncertain marks the middle band: located, but far enough from an exact
	// match that the UI should say the text may have moved.
	Uncertain bool `json:"uncertain,omitempty"`
	// Replacement is the suggested substitute for Quote. A pointer so a
	// deletion suggestion ("") is distinguishable from none.
	Replacement *string `json:"replacement,omitempty"`
}

// spanWire is a [start, end) byte range into the entity body.
type spanWire struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type commentWire struct {
	ID        string     `json:"id"`
	Author    string     `json:"author"`
	CreatedAt string     `json:"created_at"`
	Anchor    anchorWire `json:"anchor"`
	Body      string     `json:"body"`
	Resolved  bool       `json:"resolved"`
	// Detached reports that the anchor no longer names anything on the target.
	// A soft condition per DEC-HWZHA: the comment is still returned and still
	// readable, flagged so the UI can show it as orphaned rather than pretend
	// it points somewhere.
	Detached bool `json:"detached,omitempty"`
	// Editable and Deletable are UI hints mirroring `_actions`: the server
	// re-authorizes every write, so a client that ignores them gains nothing.
	Editable  bool `json:"editable"`
	Deletable bool `json:"deletable"`
	// Acceptable reports that the suggestion could be applied to the current
	// body: it has one, the comment is open, and the quote still locates
	// unambiguously. It does NOT include the entity update right; a client
	// combines it with the entity's `_actions.update`, and the accept route
	// re-checks everything.
	Acceptable bool `json:"acceptable,omitempty"`
}

func (h *commentsHandler) listComments(
	w http.ResponseWriter, r *http.Request, addr commentAddress,
) {
	list, fail := h.listOp(r.Context(), addr)
	if fail != nil {
		fail.writeHTTP(w, r)
		return
	}
	out := make([]commentWire, 0, len(list))
	for _, l := range list {
		out = append(out, commentWire{
			ID:         l.view.ID,
			Author:     l.view.Author,
			CreatedAt:  l.view.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Anchor:     l.anchor,
			Body:       l.view.Body,
			Resolved:   l.view.Resolved,
			Detached:   l.view.Detached,
			Editable:   l.view.Editable,
			Deletable:  l.view.Deletable,
			Acceptable: l.view.Acceptable,
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
		// Quote is the selected body text, for a `text` anchor.
		//
		// The client sends RENDERED text, not offsets: it renders markdown, so
		// its coordinates are not the source's. The server maps the quote back
		// to source through the AST and derives the stored descriptors itself,
		// so a caller cannot persist context that disagrees with the entity.
		Quote string `json:"quote"`
		// QuotePrefix and QuoteSuffix are the rendered text immediately around
		// the selection. They select among REAL occurrences of the quote and
		// can never introduce a location, since the quote must still be found.
		//
		// Without them a repeated quote resolves to the first occurrence — how
		// a comment on "Geordend" came to highlight "Ongeordend".
		QuotePrefix string `json:"quote_prefix"`
		QuoteSuffix string `json:"quote_suffix"`
		// Replacement suggests substitute text for the quote (text anchors
		// only). Null or absent means no suggestion; "" suggests deletion.
		Replacement *string `json:"replacement"`
	} `json:"anchor"`
	Body string `json:"body"`
}

func (h *commentsHandler) addComment(
	w http.ResponseWriter, r *http.Request, addr commentAddress,
) {
	ctx := r.Context()

	target, ent, fail := h.addGate(ctx, addr)
	if fail != nil {
		fail.writeHTTP(w, r)
		return
	}

	var req addCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", "")
		return
	}

	created, fail := h.addOp(ctx, target, ent, comments.AddInput{
		Kind:        comments.AnchorKind(req.Anchor.Kind),
		Ref:         req.Anchor.Ref,
		Quote:       req.Anchor.Quote,
		QuotePrefix: req.Anchor.QuotePrefix,
		QuoteSuffix: req.Anchor.QuoteSuffix,
		Replacement: req.Anchor.Replacement,
		Body:        req.Body,
	})
	if fail != nil {
		fail.writeHTTP(w, r)
		return
	}

	writeV1JSON(w, http.StatusCreated, commentWire{
		ID:        created.ID,
		Author:    created.Author,
		CreatedAt: created.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Anchor:    newAnchorWire(created.Anchor),
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
	w http.ResponseWriter, r *http.Request, addr commentAddress, commentID string,
) {
	ctx := r.Context()

	target, existing, fail := h.mutationGate(ctx, addr, commentID, mutationUpdate)
	if fail != nil {
		fail.writeHTTP(w, r)
		return
	}

	var req updateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "Malformed JSON body", "")
		return
	}

	if fail := h.updateOp(ctx, target, existing, commentID, req.Body, req.Resolved); fail != nil {
		fail.writeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *commentsHandler) deleteComment(
	w http.ResponseWriter, r *http.Request, addr commentAddress, commentID string,
) {
	if fail := h.deleteOp(r.Context(), addr, commentID); fail != nil {
		fail.writeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseCommentAddress parses a target type and its address (`ID` or
// `ID@face`), reporting false for anything unsafe to hand to storage. The
// route and [MCPComments] share it.
func parseCommentAddress(typeName, address string) (commentAddress, bool) {
	ref, err := entity.ParseRef(address)
	if !isSafePathSegment(typeName) || !isSafeStateRefSegment(address) || err != nil {
		return commentAddress{}, false
	}
	return commentAddress{typeName: typeName, ref: ref}, true
}

// commentAddress is the parsed target of a comments request: the entity type
// and its address, face included.
type commentAddress struct {
	typeName string
	ref      entity.Ref
}

// isSafeStateRefSegment reports whether a path segment is a usable entity
// reference, with or without a face ("TKT-1", "TKT-1@draft").
//
// A separate check rather than widening [isSafePathSegment]: that helper guards
// every other route, and '@' is only meaningful where a face may legitimately
// appear. Both halves are validated on their own terms, so nothing unsafe
// reaches storage — the face half by the entity package's own grammar, which
// is narrower than the id's.
func isSafeStateRefSegment(s string) bool {
	base, face, found := strings.Cut(s, entity.StateRefSeparator)
	if !isSafePathSegment(base) {
		return false
	}
	if !found {
		return true
	}
	// entity.ParseFace is the only sanctioned constructor from external input,
	// and rejects anything outside the declared face grammar.
	_, err := entity.ParseFace(face)
	return err == nil
}

// newAnchorWire projects a stored anchor onto the wire WITHOUT resolving it.
//
// Used on create, where the client already knows where it selected: a text
// anchor's range is resolved per read, so echoing one here would be a second
// code path producing the same number.
func newAnchorWire(a comments.Anchor) anchorWire {
	v := a.View()
	return anchorWire{Kind: string(v.Kind), Ref: v.Ref, Quote: v.Quote, Replacement: v.Replacement}
}

// resolveAnchor projects a stored anchor onto the wire and reports whether it
// is currently detached, by [comments.Detached]'s rule.
//
// A section anchor is never flagged: its ref resolves against the view config,
// not the entity, so this per-entity view cannot tell a missing section from
// a present one, and guessing would badge every section comment. A text
// anchor that still locates also gets a fresh range plus a confidence band.
//
// An entity that could not be loaded flags nothing: failing to read must not
// make every comment look orphaned.
func resolveAnchor(ctx anchorContext, a comments.Anchor) (anchorWire, bool) {
	out := newAnchorWire(a)
	if !ctx.loaded {
		return out, false
	}
	if comments.Detached(a, ctx.properties, ctx.body) {
		return out, true
	}

	switch a.Kind {
	case comments.AnchorText:
		m := ctx.body.ResolveText(a.Text)
		start, end := m.Start, m.End
		out.Start, out.End = &start, &end
		segments := make([]spanWire, len(m.Segments))
		for i, sp := range m.Segments {
			segments[i] = spanWire(sp)
		}
		out.Segments = &segments
		out.Confidence = m.Confidence
		out.Uncertain = m.Uncertain
		return out, false

	default:
		return out, false
	}
}

// acceptResponse is the accept route's success body.
//
// Content is the body the write submitted, so the SPA can show the change at
// once. A backend that reformats on write (fsstore reflows to 80 columns) may
// store it differently; the SPA reloads the view for the stored form. Warnings
// are the soft validation findings of the write (DEC-HWZHA).
type acceptResponse struct {
	Content  string           `json:"content"`
	Warnings []entity.Warning `json:"warnings,omitempty"`
}

// commentAccept is the route for [commentsHandler.acceptOp].
func (h *commentsHandler) commentAccept(
	w http.ResponseWriter, r *http.Request, addr commentAddress, commentID string,
) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if fail := h.readOnlyFailure(); fail != nil {
		fail.writeHTTP(w, r)
		return
	}
	r = h.withProvision(r)

	accepted, fail := h.acceptOp(r.Context(), addr, commentID)
	if fail != nil {
		fail.writeHTTP(w, r)
		return
	}
	writeV1JSON(w, http.StatusOK, acceptResponse(accepted))
}
