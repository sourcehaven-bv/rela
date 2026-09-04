package dataentry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// commentsHandler owns the commentary routes.
//
// Extracted from App (the exportHandler pattern, TKT-JF5JI8) to keep App under
// its plimsoll method load line. It closes over the swappable collaborators
// rather than capturing their values, so a metamodel live-reload or an ACL swap
// is picked up without rebuilding the handler.
type commentsHandler struct {
	// svc is nil when the project declares no enabled `comments:` block. Nil IS
	// the "feature absent" signal: the routes 404 and no storage is touched.
	svc *comments.Service

	meta          func() *metamodel.Metamodel
	acl           func() acl.ACL
	visibleReader visibleReader
}

// newCommentsHandler builds the handler over app's collaborators.
//
// A handler is returned even when commenting is disabled — the nil service is
// checked per request, so the route stays registered and answers a JSON 404
// rather than falling through to the stdlib's unregistered-route handling.
func newCommentsHandler(app *App) *commentsHandler {
	return &commentsHandler{
		meta:          app.Meta,
		acl:           func() acl.ACL { return app.acl },
		visibleReader: app.visibleReader,
	}
}

// SetComments installs the commentary service.
//
// Nil (the default) means the project declares no `comments:` block: the
// comment routes 404 and no storage is touched, so a project without the block
// is indistinguishable from one built before the feature existed.
func (a *App) SetComments(svc *comments.Service) { a.comments.svc = svc }

// commentPolicy returns the metamodel's comment policy.
//
// Read live off the current metamodel rather than captured at construction:
// the watcher swaps the metamodel atomically on a file change, so a captured
// policy would keep answering from the config the server booted with.
func (h *commentsHandler) commentPolicy() metamodel.CommentPolicy {
	return metamodel.NewCommentPolicy(h.meta())
}

// commentPermissionGate adapts the ACL's per-entity permission resolver to the
// narrow gate the comments package declares.
//
// It uses [acl.Request.HoldsPermissionForEntity] — the subject-aware sibling of
// HoldsPermission — so a `comment:*` permission conferred by an ownership
// relation TO THE COMMENTED-ON ENTITY is honored, not just a global grant. That
// is what lets "the assignee may comment on their own ticket" be expressed
// without any special case here.
type commentPermissionGate struct{ policyActive bool }

func (g commentPermissionGate) HoldsPermission(ctx context.Context, subjectID, permission string) bool {
	req := acl.FromContext(ctx)
	if req == nil {
		// Mirrors the statemachine transition guard (appbuild/transitions.go):
		// inert when no policy exists at all, fail CLOSED when one does but the
		// Request is missing — a served path that lost its middleware must not
		// silently open commenting.
		return !g.policyActive
	}
	return req.HoldsPermissionForEntity(ctx, subjectID, permission)
}

// commentReadGate adapts the request read gate to the comments package's floor
// check.
//
// Errors collapse to "denied": this gate feeds an authorization decision, and a
// gate that cannot answer must not be read as a yes. The HTTP layer separately
// runs gateReadOrNotFound, which surfaces the error properly, so a genuine
// backend fault is still reported rather than silently becoming a 403.
type commentReadGate struct{}

func (commentReadGate) CanRead(ctx context.Context, entityType, entityID string) bool {
	ok, err := readGateFromContext(ctx).PermitsRead(ctx, entityType, entityID)
	return err == nil && ok
}

// commentAuthorizer builds the per-request authorizer.
//
// Constructed per call rather than stored: it closes over nothing
// request-scoped itself, but the gates it holds read the ACL from ctx, and a
// cached authorizer would invite someone to give it request state later.
func (h *commentsHandler) commentAuthorizer() *comments.Authorizer {
	active := h.aclPolicyActive()
	return comments.NewAuthorizer(commentPermissionGate{policyActive: active}, commentReadGate{}, active)
}

// aclPolicyActive reports whether per-principal comment permissions are
// enforced.
//
// True only for a declarative policy — the deployment that actually has roles
// and permissions to evaluate. [acl.NopACL] and [acl.ReadOnlyACL] both leave
// this false: neither carries a role model, so demanding `comment:read` under
// them would deny every request rather than authorize anything.
//
// ReadOnlyACL's write refusal is enforced separately by
// commentWritesPermitted, NOT by pretending it is a permission policy:
// it is a process-wide switch, and conflating the two made read-only instances
// refuse comment READS as well.
func (h *commentsHandler) aclPolicyActive() bool {
	_, ok := h.acl().(*acl.Declarative)
	return ok
}

// commentWritesPermitted reports whether this instance accepts comment writes
// at all.
//
// Separate from the per-request permission checks because ReadOnlyACL is a
// process-wide refusal, not a permission a principal could hold: no grant in
// any acl.yaml should make a read-only instance writable.
func (h *commentsHandler) commentWritesPermitted() bool {
	switch h.acl().(type) {
	case acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return false
	default:
		return true
	}
}

// anchorContext is what a read needs to decide how each anchor currently
// resolves: the property names that exist, and the body that text anchors are
// located within.
//
// Loaded ONCE per list request. Resolving text anchors needs the entity body,
// and re-reading it per comment would be N redacted reads for N comments.
type anchorContext struct {
	// properties is nil when the entity could not be loaded, which means "do
	// not flag anything as detached" rather than "nothing exists".
	properties map[string]bool
	// body is the entity content text anchors resolve against. Empty when the
	// entity could not be read.
	body string
	// loaded distinguishes "read the entity, it has an empty body" from "could
	// not read the entity at all" — only the latter suppresses detach flags.
	loaded bool
}

// liveAnchors returns the set of anchor refs that currently resolve on the
// target, or nil when the set could not be determined.
//
// A nil result means "do not flag anything as detached": failing to load the
// entity must not make every comment on it look orphaned. That asymmetry is
// deliberate — a false "detached" badge on a healthy thread is more alarming,
// and less recoverable by the user, than a missing one.
//
// Only PROPERTY anchors are resolved, so the caller must not consult this for a
// section anchor: a section ref names an operator-authored view heading that
// lives in data-entry.yaml, not on the entity, and its absence from this set
// means "not a property", not "gone".
func (h *commentsHandler) liveAnchors(ctx context.Context, target comments.Target) anchorContext {
	def, ok := h.meta().GetEntityDef(target.Type)
	if !ok {
		return anchorContext{}
	}
	ent, found, err := h.visibleReader.getVisible(ctx, target.Type, target.ID)
	if err != nil || !found {
		return anchorContext{}
	}

	refs := make(map[string]bool, len(def.Properties)+len(ent.Properties))
	// Declared properties count as live even when unset: a comment on an empty
	// field is about a field that exists, and is not orphaned.
	for name := range def.Properties {
		refs[name] = true
	}
	// Properties present on the entity but absent from the schema (hand-edited
	// frontmatter) also count: the value the comment discusses is right there.
	for name := range ent.Properties {
		refs[name] = true
	}
	// The body is the REDACTED entity's content, so a text anchor can only ever
	// resolve against text this principal may already read.
	return anchorContext{properties: refs, body: ent.Content, loaded: true}
}

// buildTextAnchor derives a text anchor's descriptors from the entity's own
// body, given the quote the client selected.
//
// The client supplies the quote and which occurrence it meant; everything else
// (prefix, suffix, sentence, heading, paragraph index) is computed here. That
// asymmetry is deliberate and is the security-relevant part: descriptors
// sourced from the request could describe context that does not exist in the
// entity, so a later resolve would land on text the commenter never selected.
//
// The body read goes through the visibility wrapper, so a principal can only
// anchor to text it may already read.
func (h *commentsHandler) buildTextAnchor(
	ctx context.Context, target comments.Target, quote string, occurrence int,
) (*comments.TextAnchor, error) {
	quote = strings.TrimSpace(quote)
	if len([]rune(quote)) < comments.MinQuoteRunes {
		return nil, fmt.Errorf("selected text must be at least %d characters", comments.MinQuoteRunes)
	}
	if len(quote) > comments.MaxQuoteBytes {
		return nil, fmt.Errorf("selected text exceeds %d bytes", comments.MaxQuoteBytes)
	}

	ent, found, err := h.visibleReader.getVisible(ctx, target.Type, target.ID)
	if err != nil || !found {
		return nil, errors.New("could not read the entity body")
	}

	start := nthIndex(ent.Content, quote, occurrence)
	if start < 0 {
		// The selection does not exist in the stored body. Usually a stale tab:
		// the entity changed under the user between render and submit.
		return nil, errors.New("the selected text was not found in the current body")
	}

	return comments.NewTextAnchor(ent.Content, start, start+len(quote))
}

// nthIndex returns the byte offset of the nth (0-based) occurrence of sub in s,
// or -1. A negative or out-of-range n falls back to the first occurrence rather
// than failing: which occurrence was meant is a UI nicety, and refusing the
// whole comment over it would be a worse trade than anchoring to the first.
func nthIndex(s, sub string, n int) int {
	if n <= 0 {
		return strings.Index(s, sub)
	}
	offset := 0
	for i := 0; i <= n; i++ {
		idx := strings.Index(s[offset:], sub)
		if idx < 0 {
			if i == 0 {
				return -1
			}
			// Fewer occurrences than asked for: use the last one found.
			return strings.LastIndex(s, sub)
		}
		offset += idx
		if i == n {
			return offset
		}
		offset += len(sub)
	}
	return -1
}
