package dataentry

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
)

// maybeProvision implements `unmatched_principal: provision`: when the
// request's principal was verified but resolved to no user entity, it creates
// a bare stub user under system:provisioner, re-stamps ctx to that entity with
// a fresh acl.Request and read gate, and returns the ctx the handler must
// ADOPT. Every entity-write handler calls it via withProvision. A reserved
// `system:*` subject is never persisted as a stub's join key. On every other
// path ctx is returned unchanged. It never fails the request: on error the
// ORIGINAL ctx is returned and normal authorization gates the write.
func maybeProvision(
	ctx context.Context, d *acl.Declarative, m entityProvisioner, meta metaView,
) context.Context {
	if d == nil {
		return ctx
	}
	if req, err := d.RequestFrom(ctx); err != nil || !req.UnmatchedVerified() {
		return ctx
	}
	pol := d.Policy()
	if pol.EffectiveUnmatchedPrincipal() != acl.UnmatchedProvision {
		return ctx
	}

	ctx = readtime.Ensure(ctx, nowFunc)
	p := principal.From(ctx)
	sub := p.User

	if principal.IsReserved(sub) {
		slog.WarnContext(ctx, "dataentry: provision: refusing to provision a reserved principal",
			"sub", sub, "user_type", pol.UserEntityType)
		return ctx
	}

	provCtx, err := acl.BindRequest(principal.With(ctx, principal.Principal{
		User: principal.UserProvisioner,
		Tool: principal.ToolProvisioner,
	}), d)
	if err != nil {
		slog.Warn("dataentry: provision: could not resolve the provisioner identity",
			"sub", sub, "err", err)
		return ctx
	}
	stub := buildStubEntity(pol, meta, p)
	_, err = m.CreateEntity(provCtx, stub, entity.CreateOptions{})
	switch {
	case err == nil:
	case errors.Is(err, entitymanager.ErrEntityAlreadyExists), entitymanager.IsUniqueViolation(err):
	default:
		slog.Warn("dataentry: provision: stub create failed; proceeding as unmatched",
			"sub", sub, "user_type", pol.UserEntityType, "err", err)
		return ctx
	}

	provisioned, req, rerr := access.Restamp(ctx, d)
	if rerr != nil {
		slog.Warn("dataentry: provision: re-stamp after create failed; proceeding as unmatched",
			"sub", sub, "err", rerr)
		return ctx
	}
	gate, gerr := newACLReadGate(req)
	if gerr != nil { // coverage-ignore: Restamp returns a non-nil Request whenever its error is nil
		slog.Warn("dataentry: provision: read-gate rebuild failed", "sub", sub, "err", gerr)
		return ctx
	}
	return withReadGate(provisioned, gate)
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

// entityProvisioner is the narrow write surface maybeProvision needs: creating
// the stub entity for a verified-but-unmatched principal. Defined at the call
// site (CLAUDE.md consumer-side interfaces), same rationale as [metaView]
// below — provisioning creates one entity and does nothing else, so it has no
// business holding a delete or a rename (TKT-IVSJV6).
//
// The create still routes through the manager, so the stub is validated,
// audited and runs automations exactly like any other entity.
type entityProvisioner interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
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
