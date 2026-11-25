package dataentry

import (
	"context"
	"errors"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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

	// The accept route (TKT-S5C0K3) writes the entity body, so it needs what
	// every other mutation handler has: the RAW reader for the splice base
	// and version token, the manager to apply the patch, and the provision
	// seam.
	reader    entityReader
	patcher   suggestionPatcher
	provision func(context.Context) context.Context
	// audit records an accept the ACL refuses before the manager is reached.
	audit func() audit.Audit
}

// suggestionPatcher is the one write the accept route performs.
//
// Declared here, at its only consumer, so the comments handler depends on a
// single manager method rather than the whole write surface.
type suggestionPatcher interface {
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
}

// newCommentsHandler builds the handler over app's collaborators.
//
// A handler is returned even when commenting is disabled — the nil service is
// checked per request, so the route stays registered and answers a JSON 404
// rather than falling through to the stdlib's unregistered-route handling.
func newCommentsHandler(app *App, patcher suggestionPatcher) *commentsHandler {
	return &commentsHandler{
		meta:          app.Meta,
		acl:           func() acl.ACL { return app.acl },
		visibleReader: app.visibleReader,
		reader:        app.reader,
		patcher:       patcher,
		provision:     newProvisionSeam(app),
		audit:         func() audit.Audit { return app.auditSink },
	}
}

// withProvision runs the provision seam, returning the request with the
// re-stamped context. The same shape as the other write handlers, which
// provision_seam_invariant_test.go holds every write route to.
func (h *commentsHandler) withProvision(r *http.Request) *http.Request {
	if h.provision != nil {
		return r.WithContext(h.provisioned(r.Context()))
	}
	return r
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
//
// active reports whether a declarative policy is configured; without one the
// gate is inert. policy is that policy. Nil policy while active: accepted —
// a typed-nil policy is a wiring bug, and the gate denies.
type commentPermissionGate struct {
	policy *acl.Declarative
	active bool
}

func (g commentPermissionGate) HoldsPermission(ctx context.Context, subjectID, permission string) bool {
	if !g.active {
		return true
	}
	if g.policy == nil {
		return false
	}
	req, err := g.policy.RequestFrom(ctx)
	if err != nil {
		return false
	}
	return req.HoldsPermissionForEntity(ctx, subjectID, permission)
}

// commentReadGate adapts the request read gate to the comments package's floor
// check.
//
// Errors collapse to "denied": this gate feeds an authorization decision, and a
// gate that cannot answer must not be read as a yes. The HTTP layer separately
// runs gateReadOr, which surfaces the error properly, so a genuine
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
	policy, active := h.aclPolicy()
	return comments.NewAuthorizer(commentPermissionGate{policy: policy, active: active}, commentReadGate{}, active)
}

// aclPolicy returns the declarative policy, and whether per-principal comment
// permissions are enforced. Only a declarative policy is active; a typed-nil
// one is active with a nil policy, so the gate fails closed. [acl.NopACL] and
// [acl.ReadOnlyACL] are inactive because they carry no role model.
// ReadOnlyACL's write refusal is enforced by commentWritesPermitted.
func (h *commentsHandler) aclPolicy() (policy *acl.Declarative, active bool) {
	policy, _ = h.acl().(*acl.Declarative)
	return policy, access.ModeOf(h.acl()).PolicyConfigured()
}

// commentWritesPermitted reports whether this instance accepts comment writes
// at all.
//
// Separate from the per-request permission checks because ReadOnlyACL is a
// process-wide refusal, not a permission a principal could hold: no grant in
// any acl.yaml should make a read-only instance writable.
func (h *commentsHandler) commentWritesPermitted() bool {
	return !access.ModeOf(h.acl()).RefusesWrites()
}

// anchorContext is what a read needs to decide how each anchor currently
// resolves: the property names that exist, and the body that text anchors are
// located within.
//
// Loaded ONCE per list request. Resolving text anchors needs the entity body,
// and re-reading it per comment would be N redacted reads for N comments; the
// body is also prepared once, so matching and highlight segmenting do not
// re-scan it per comment.
type anchorContext struct {
	// properties is nil when the entity could not be loaded, which means "do
	// not flag anything as detached" rather than "nothing exists".
	properties map[string]bool
	// body is the entity content text anchors resolve against. Nil when the
	// entity could not be read.
	body *comments.Body
	// loaded distinguishes "read the entity, it has an empty body" from "could
	// not read the entity at all" — only the latter suppresses detach flags.
	loaded bool
}

// liveAnchors returns the set of property anchor refs that resolve on the
// target, or nil when it cannot be determined; nil means "flag nothing as
// detached". Section anchors are not resolved, so callers must not consult
// this for them. ent is the entity the target gate resolved, passed in so the
// anchors resolve against the face the thread belongs to.
func (h *commentsHandler) liveAnchors(target comments.Target, ent *entity.Entity) anchorContext {
	def, ok := h.meta().GetEntityDef(target.Type)
	if !ok || ent == nil {
		return anchorContext{}
	}

	refs := comments.LiveProperties(def.Properties, ent.Properties)
	return anchorContext{properties: refs, body: comments.NewBody(ent.Content), loaded: true}
}

// buildTextAnchor builds a text anchor in ent's body with
// [comments.BuildTextAnchor]. ent is the row the request gate resolved
// through the visibility wrapper, so a principal can only anchor to text it
// may already read, and only within the face the thread belongs to.
func buildTextAnchor(ent *entity.Entity, quote, prefix, suffix string) (*comments.TextAnchor, error) {
	if ent == nil {
		return nil, errors.New("could not read the entity body")
	}
	return comments.BuildTextAnchor(ent.Content, quote, prefix, suffix)
}
