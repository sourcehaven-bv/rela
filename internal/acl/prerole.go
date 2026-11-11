package acl

import (
	"context"
	"fmt"
	"log/slog"
)

// Pre-role write checks.
//
// A write meets two kinds of checks. The role grants say what the principal
// may do. The pre-role checks say whether this principal, through this
// client, may write at all, whatever its roles:
//
//   - a usable [Request] on ctx: none is [ErrNoIdentity], one from another
//     Declarative is [ErrForeignIdentity], one bound to another principal is
//     [ErrIdentityMismatch];
//   - `unmatched_principal: reject` for an unmatched verified principal;
//   - the client ceiling (client_baselines and scope_grants) on the verb and
//     type, and for a conversion on the destination type.
//
// [Declarative.AuthorizeWrite] applies them through [openWrite] and
// [entityPreRoleDenial]. [AuthorizePreRoleWrite] applies the same functions
// for a write whose authority comes from something other than a role grant,
// so a check added here reaches both.
//
// Two checks are not here on purpose. The `requires_permission` gate of a
// role relation and the `relation_grants` path ask what the principal's roles
// grant, so they are role checks. Read-only mode is not part of a
// Declarative: it is [ReadOnlyACL], which [AuthorizePreRoleWrite] asks
// directly.

// openWrite returns the Request a write evaluates with, or the denial when the
// write fails a check that holds before any subject is looked at.
//
// `unmatched_principal` runs before role evaluation because it is a
// deployment posture ("unknown identities don't mutate"), not a role
// decision. Every entity and relation write reaches it: role-granted writes
// through [Declarative.AuthorizeWrite], and elevated data-entry action writes
// through [AuthorizePreRoleWrite] (TKT-0C3II2, TKT-JA5SABG).
func openWrite(ctx context.Context, d *Declarative) (*Request, *Decision) {
	r, err := d.RequestFrom(ctx)
	if err != nil {
		// The error text is fixed (sentinels only), so it is safe to return
		// to a caller; RequestFrom logs any principal ids.
		return nil, &Decision{
			Allow:    false,
			RuleKind: "role-grant",
			RuleID:   "-",
			Reason:   fmt.Sprintf("acl: no request scope: %v", err),
			Cause:    err,
		}
	}
	if !r.UnmatchedVerified() {
		return r, nil
	}
	switch d.policy.effectiveUnmatchedPrincipal() {
	case UnmatchedReject:
		return nil, &Decision{
			Allow:    false,
			RuleKind: "unmatched-principal",
			RuleID:   UnmatchedReject,
			Reason:   "verified principal resolves to no user entity; writes are rejected",
		}
	case UnmatchedProvision:
		// Provision is implemented at the data-entry write seam (maybeProvision,
		// TKT-ANUJDS): a successful provision binds a new Request on the
		// provisioned identity, which is not unmatched, so the write never
		// reaches here still-flagged. Arriving here under provision means
		// provisioning did NOT fire or failed — a non-data-entry transport (this
		// gate is transport-agnostic), or a create/re-resolve error the seam
		// logged. Fall through to normal evaluation (the principal's asserted
		// roles still apply, TKT-RP3X3Q); warn once so a stuck provision
		// deployment is visible without a per-request flood.
		d.provisionWarn.Do(func() {
			slog.Warn("acl: unmatched_principal: provision reached the write-authz gate " +
				"still unmatched; provisioning did not fire (non-data-entry transport) or " +
				"failed (see earlier provision warnings); evaluating the roles it holds without a user entity")
		})
	}
	return r, nil
}

// entityPreRoleDenial returns the client-ceiling denial for op on the entity
// s names, or nil when the ceiling permits it. A conversion also needs the
// ceiling's create axis on its destination type.
func entityPreRoleDenial(r *Request, attrs []RoleAttribution, op Op, s EntitySubject) *Decision {
	if deny := r.ceilingDenial(attrs, op, s.typ); deny != nil {
		return deny
	}
	if s.family && op == OpConvert {
		return r.conversionCeilingDenial(attrs, s.typ, s.convertTo)
	}
	return nil
}

// preRoleWriter is an ACL that can decide a write on its pre-role checks
// alone. [Declarative] is one.
type preRoleWriter interface {
	AuthorizePreRoleWrite(ctx context.Context, req WriteRequest) Decision
}

// AuthorizePreRoleWrite decides req on every check a's AuthorizeWrite makes
// that does not come from a role grant. It is for a write whose authority is
// granted elsewhere, such as the `elevated_updates:` of a data-entry action:
// elevation lifts the role grants, never the identity, policy-posture or
// client limits.
//
// An ACL with its own AuthorizePreRoleWrite method answers through it. Any
// other ACL answers with its full AuthorizeWrite: [ReadOnlyACL] denies,
// [NopACL] allows, and an ACL that cannot separate its checks gets no
// elevation.
//
// Nil: a nil a, or a nil *Declarative, is a deny; an unwired ACL must not read
// as "no checks".
func AuthorizePreRoleWrite(ctx context.Context, a ACL, req WriteRequest) Decision {
	if d, ok := a.(*Declarative); a == nil || (ok && d == nil) {
		return Decision{
			Allow:    false,
			RuleKind: "role-grant",
			RuleID:   "-",
			Reason:   "acl: no write policy is wired",
		}
	}
	if p, ok := a.(preRoleWriter); ok {
		return p.AuthorizePreRoleWrite(ctx, req)
	}
	return a.AuthorizeWrite(ctx, req)
}

// AuthorizePreRoleWrite applies the checks listed at the top of this file and
// allows when they pass; see the package function [AuthorizePreRoleWrite]. A
// [RelationSubject] gets the full [Declarative.AuthorizeWrite], because no
// elevation grants relation writes.
func (d *Declarative) AuthorizePreRoleWrite(ctx context.Context, req WriteRequest) Decision {
	r, deny := openWrite(ctx, d)
	if deny != nil {
		return *deny
	}
	s, isEntity := req.Subject.(EntitySubject)
	if !isEntity {
		return r.AuthorizeWrite(ctx, req)
	}
	attrs := r.Globals().Attributions
	if deny := entityPreRoleDenial(r, attrs, req.Op, s); deny != nil {
		return *deny
	}
	return Decision{Allow: true, RuleKind: "pre-role", RuleID: "-", Attributions: attrs}
}
