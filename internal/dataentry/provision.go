package dataentry

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// maybeProvision implements `unmatched_principal: provision` (TKT-ANUJDS): when
// the request's principal was cryptographically verified but resolved to no
// user entity, it lazily creates a bare stub user entity so the principal gains
// a graph identity, then re-stamps ctx to the newly-created entity and returns
// it for the handler to adopt.
//
// # Where it is called
//
// At the top of every entity-write handler, INSIDE the writeMu that handler
// already holds (handleV1CreateEntity, ...Update, ...Delete, the relation
// handlers, sync ApplyEntity, the Lua-action handler, attachment put/delete).
// Two properties make that the right seam rather than a wrapping middleware:
//
//   - writeMu is a plain non-reentrant sync.Mutex taken at the top of all ~14
//     mutation handlers; a middleware that also took it would self-deadlock,
//     and routes are not method-split at registration (GET and the mutating
//     verbs share the /api/v1/ catch-all), so "wrap only writes" is not even
//     cleanly registrable. Calling here — already under the lock — serializes
//     the provision create against every other mutation FOR FREE (no second
//     lock), so two concurrent first-writes from one principal cannot both
//     create in the same process.
//   - Every write path reaches this helper, so provision covers CRUD, sync,
//     action, and attachment uniformly — the same anti-bypass property reject
//     gets from the single AuthorizeWrite choke point.
//
// # What it returns
//
// The ctx the handler must ADOPT (r = r.WithContext(returned)) before calling
// the manager and before its response-shaping read-back. On the provisioned
// path the returned ctx carries: (1) a principal re-stamped to the resolved
// entity ID (so manager.CreateEntity/UpdateEntity authorizes and audits to the
// real entity, not the anonymous principal — RR-VI9XMY gap 1), and (2) a fresh
// acl.Request + read gate built on that principal (so the write handler's
// read-back through h.reader/h.serializer cannot redact or 404 the entity it
// just created — RR-VI9XMY gap 2). On every non-provision path it returns ctx
// UNCHANGED, so a normal write pays only a flag check.
//
// It never fails the request: a provision error is logged and the ORIGINAL ctx
// is returned, so the write proceeds under the unmatched principal and is
// gated/rejected by normal authorization — fail-closed, never fail-open to a
// forged identity.
func maybeProvision(
	ctx context.Context, d *acl.Declarative, m entitymanager.EntityManager, meta metaView,
) context.Context {
	// Fast path: only a verified-but-unmatched principal under a provision
	// policy does anything. Both are cheap ctx/field reads.
	if d == nil || !acl.UnmatchedVerifiedFrom(ctx) {
		return ctx
	}
	pol := d.Policy()
	if pol.EffectiveUnmatchedPrincipal() != acl.UnmatchedProvision {
		return ctx
	}

	p := principal.From(ctx)
	sub := p.User // the verified subject; the lookup found no entity for it

	// Never persist a reserved identity as a stub's principal_property join key
	// (TKT-9PCL7D). The boundary checks in stampAuditPrincipal/verifiedPrincipal
	// mean a `system:*` subject cannot reach here today, but the invariant "a
	// reserved name never becomes durable graph state" must not depend on an
	// upstream check: a stub carrying `system:scheduler` as its join key would
	// make ResolvePrincipal map that name to a real entity. Belt and braces on
	// the write side, matching the read-side rejection.
	//
	// This gates the SUBJECT, not the actor: the create below is stamped
	// UserProvisioner, which is itself reserved and must keep working.
	if principal.IsReserved(sub) {
		slog.WarnContext(ctx, "dataentry: provision: refusing to provision a reserved principal",
			"sub", sub, "user_type", pol.UserEntityType)
		return ctx
	}

	// Create the stub under system:provisioner — a create-user-only identity, so
	// the write authorizes and audits to it while it cannot touch anything but a
	// fresh stub of the user type (bare-stub containment, RR-28SCW3).
	provCtx := principal.With(ctx, principal.Principal{
		User: principal.UserProvisioner,
		Tool: principal.ToolProvisioner,
	})
	stub := buildStubEntity(pol, meta, p)
	_, err := m.CreateEntity(provCtx, stub, entity.CreateOptions{})
	switch {
	case err == nil:
		// created
	case errors.Is(err, entitymanager.ErrEntityAlreadyExists):
		// A concurrent first-write (another process, or the IdP webhook) already
		// provisioned this sub. The unique `principal_property` makes that a
		// clean conflict: fall through and re-resolve to whatever exists now.
	default:
		slog.Warn("dataentry: provision: stub create failed; proceeding as unmatched",
			"sub", sub, "user_type", pol.UserEntityType, "err", err)
		return ctx
	}

	// Re-resolve the (now-existing) entity and re-stamp ctx to it, rebuilding the
	// ACL request + read gate on the resolved principal so both the manager call
	// and the response-shaping read see the new entity.
	id, rerr := d.ResolvePrincipal(ctx, sub)
	if rerr != nil || id == "" {
		// Ambiguous/errored re-resolve (e.g. two stubs raced onto the same sub on
		// a backend without atomic uniqueness) — log and proceed as unmatched
		// rather than guess. The write is then normally authorized.
		slog.Warn("dataentry: provision: re-resolve after create found no single entity",
			"sub", sub, "err", rerr)
		return ctx
	}

	resolved := principal.Verified(id, p.Tool, p.OrgID(), p.OrgSlug(), p.Roles()).WithEmail(p.Email())
	resolved.RawUser = sub
	// The principal now resolves to a real entity — clear the unmatched flag so
	// the triggering write is judged on the resolved identity's roles, not
	// re-caught by the unmatched gate in AuthorizeWrite.
	provisioned := acl.WithMatchedVerified(principal.With(ctx, resolved))

	req, ferr := d.ForPrincipal(resolved)
	if ferr != nil {
		slog.Warn("dataentry: provision: ForPrincipal on resolved principal failed",
			"id", id, "err", ferr)
		return ctx
	}
	gate, gerr := newACLReadGate(req)
	if gerr != nil {
		slog.Warn("dataentry: provision: read-gate rebuild failed", "id", id, "err", gerr)
		return ctx
	}
	provisioned = acl.WithRequest(provisioned, req)
	provisioned = withReadGate(provisioned, gate)
	return provisioned
}

// buildStubEntity assembles the bare stub: the join key (principal_property =
// sub) always, plus email/org_id/org_slug ONLY when the user entity type
// declares them. Declared-only keeps the create from soft-warning on a lean
// user schema; a type that omits these simply gets an identity-only stub.
func buildStubEntity(pol *acl.Policy, meta metaView, p principal.Principal) *entity.Entity {
	props := map[string]any{pol.PrincipalProperty: p.User}
	if declared, ok := meta.PropertyDeclared(pol.UserEntityType, "email"); ok && declared && p.Email() != "" {
		props["email"] = p.Email()
	}
	if declared, ok := meta.PropertyDeclared(pol.UserEntityType, "org_id"); ok && declared && p.OrgID() != "" {
		props["org_id"] = p.OrgID()
	}
	if declared, ok := meta.PropertyDeclared(pol.UserEntityType, "org_slug"); ok && declared && p.OrgSlug() != "" {
		props["org_slug"] = p.OrgSlug()
	}
	return &entity.Entity{Type: pol.UserEntityType, Properties: props}
}

// metaView is the narrow schema query maybeProvision needs: whether a property
// is declared on an entity type. Defined at the call site (CLAUDE.md
// consumer-side interfaces) so provisioning does not depend on the full Schema.
type metaView interface {
	// PropertyDeclared reports whether entityType declares property. ok is false
	// when the entity type itself is unknown.
	PropertyDeclared(entityType, property string) (declared bool, ok bool)
}

// newProvisionSeam builds the write-handler provision closure for a. It is a
// package function, not an App method, deliberately: App sits on the god-object
// load line, so the seam lives here and is wired as a closure into every write
// handler instead of adding a method. The returned closure, given the request
// ctx, runs [maybeProvision] when the ACL is a *acl.Declarative (NopACL/
// ReadOnlyACL never provision) and returns the ctx the handler must adopt:
//
//	ctx := h.provision(r.Context()); r = r.WithContext(ctx)
//
// It reads a.acl live (not captured) so a test that swaps the ACL after
// construction is honored, matching the other closures on the write handlers.
func newProvisionSeam(a *App) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		d, ok := a.acl.(*acl.Declarative)
		if !ok || d == nil {
			return ctx
		}
		return maybeProvision(ctx, d, a.entityManager, schemaMetaView{schema: a.State})
	}
}

// schemaMetaView adapts the data-entry Schema accessor to the narrow metaView
// maybeProvision needs. Live closure over App.State so it reflects reloads.
type schemaMetaView struct {
	schema func() *Schema
}

func (v schemaMetaView) PropertyDeclared(entityType, property string) (declared, ok bool) {
	s := v.schema()
	if s == nil {
		return false, false
	}
	def, found := s.Meta.Entities[entityType]
	if !found {
		return false, false
	}
	_, declared = def.Properties[property]
	return declared, true
}
