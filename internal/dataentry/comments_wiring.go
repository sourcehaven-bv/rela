package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// SetComments installs the commentary service.
//
// Nil (the default) means the project declares no `comments:` block: the
// comment routes 404 and no storage is touched, so a project without the block
// is indistinguishable from one built before the feature existed.
func (a *App) SetComments(svc *comments.Service) { a.comments = svc }

// commentPolicy returns the metamodel's comment policy.
//
// Read live off the current metamodel rather than captured at construction:
// the watcher swaps the metamodel atomically on a file change, so a captured
// policy would keep answering from the config the server booted with.
func (a *App) commentPolicy() metamodel.CommentPolicy {
	return metamodel.NewCommentPolicy(a.Meta())
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
func (a *App) commentAuthorizer() *comments.Authorizer {
	active := a.aclPolicyActive()
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
// [App.commentWritesPermitted], NOT by pretending it is a permission policy:
// it is a process-wide switch, and conflating the two made read-only instances
// refuse comment READS as well.
func (a *App) aclPolicyActive() bool {
	_, ok := a.acl.(*acl.Declarative)
	return ok
}

// commentWritesPermitted reports whether this instance accepts comment writes
// at all.
//
// Separate from the per-request permission checks because ReadOnlyACL is a
// process-wide refusal, not a permission a principal could hold: no grant in
// any acl.yaml should make a read-only instance writable.
func (a *App) commentWritesPermitted() bool {
	switch a.acl.(type) {
	case acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return false
	default:
		return true
	}
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
func (a *App) liveAnchors(ctx context.Context, target comments.Target) map[string]bool {
	def, ok := a.Meta().GetEntityDef(target.Type)
	if !ok {
		return nil
	}
	ent, found, err := a.visibleReader.getVisible(ctx, target.Type, target.ID)
	if err != nil || !found {
		return nil
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
	return refs
}
