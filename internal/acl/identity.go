package acl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ErrPrincipalLookup wraps every failure of the `principal_property` lookup in
// [IdentityResolver.Resolve]: a backend error, or a natural key that matches
// more than one user entity. The request edge decides what to do with it; the
// data-entry middleware logs it and falls back to [IdentityResolver.ResolveDirect]
// on the raw principal, which can only lose grants.
var ErrPrincipalLookup = errors.New("acl: principal lookup failed")

// ErrNoIdentity reports that an evaluation path found no resolved identity on
// its context. Every request edge resolves the principal once and attaches the
// resulting [Request] with [WithRequest]. A path that finds none fails closed
// with this error rather than resolving the principal itself.
var ErrNoIdentity = errors.New("acl: no resolved identity on context")

// ErrForeignIdentity reports that an [Identity] was resolved by another
// [Declarative] than the one asked to evaluate it. The membership closure
// depends on the store and on the policy's membership relation, so an identity
// from another tenant or another policy load is not valid here.
var ErrForeignIdentity = errors.New("acl: identity was resolved by another policy or store")

// ErrIdentityMismatch reports that the [Request] attached to a context is
// bound to another principal than the one stamped on that context. Evaluating
// it would judge the wrong identity, so [Declarative.RequestFrom] refuses it.
// The error text carries no principal ids; the ids are logged instead.
var ErrIdentityMismatch = errors.New("acl: attached request is bound to another principal")

// MembershipReader is the one graph read the identity resolver needs: the
// targets of a relation from a node. The member-of walk is built on it.
// aclstore.Graph satisfies it.
type MembershipReader interface {
	OutgoingRelations(ctx context.Context, from, relType string) ([]string, error)
}

// Identity is a resolved principal: the full [principal.Principal] plus the
// closure of the policy's membership relation from it.
//
// It carries no roles and no graph handle. Roles are a pure function of an
// Identity and the policy, computed by [Declarative.ForIdentity]. The
// principal is kept whole, with its verified claims and scopes, because the
// client ceiling is compiled from them. Dropping a claim would remove the
// ceiling and widen access.
//
// The fields are unexported, so only an [IdentityResolver] can build one. The
// zero Identity is not valid: [Declarative.ForIdentity] rejects it.
type Identity struct {
	principal principal.Principal
	members   []string
	unmatched bool
	verified  bool
	// origin is the resolver that built this Identity. Each [Declarative]
	// owns exactly one resolver, so [Declarative.ForIdentity] compares
	// pointers to refuse an identity resolved elsewhere.
	origin *IdentityResolver
}

// Principal returns the resolved principal. After [IdentityResolver.Resolve]
// its User may be the user entity id that `principal_property` mapped the raw
// identifier to; RawUser then holds the original value. The edge stamps this
// principal on its context so the audit log and the ACL agree.
//
// Cloned so a caller cannot reach the backing arrays of the claims.
func (id Identity) Principal() principal.Principal { return id.principal.Clone() }

// Unmatched reports that the `principal_property` lookup was enabled, ran,
// and found no user entity for the principal. Only [IdentityResolver.Resolve]
// sets it. The data-entry edge combines it with "identity came from a
// verified JWT" to apply `unmatched_principal`; see
// [Identity.WithVerifiedAssertion].
func (id Identity) Unmatched() bool { return id.unmatched }

// WithVerifiedAssertion returns a copy of id that records that its principal
// came from a cryptographically verified assertion (the data-entry JWT gate).
// Only the edge knows this: a JWT principal and a proxy-header principal look
// the same on a [principal.Principal].
//
// When the identity is also [Identity.Unmatched], the [Request] opened on it
// is unmatched-verified (see [Request.UnmatchedVerified]), and
// [Declarative.AuthorizeWrite] applies the policy's `unmatched_principal`
// mode to its writes.
func (id Identity) WithVerifiedAssertion() Identity {
	id.verified = true
	return id
}

// IdentityResolver turns a stamped principal into an [Identity]. It runs the
// `principal_property` lookup and the walk over the policy's membership
// relation. It is the only code in this package that walks membership; the
// evaluator ([Declarative], [Request]) reads the closure from the Identity.
//
// Obtain one from [Declarative.IdentityResolver]. It shares the Declarative's
// policy and store, which is what makes its identities valid there.
type IdentityResolver struct {
	policy  *Policy
	members MembershipReader
	lookup  PrincipalLookup // Nil: accepted — only when the policy disables the lookup
}

func newIdentityResolver(p *Policy, m MembershipReader, l PrincipalLookup) *IdentityResolver {
	return &IdentityResolver{policy: p, members: m, lookup: l}
}

// Resolve resolves p for a transport whose identifier may be an external
// natural key (the data-entry proxy header or JWT subject). When the policy
// sets `user_entity_type` and `principal_property`, the raw p.User is looked
// up. One match replaces User with the entity id, keeps the raw value in
// RawUser, and keeps every verified claim. No match leaves p unchanged and
// sets [Identity.Unmatched]. Then the membership closure is walked.
//
// Errors:
//   - [ErrUnstampedPrincipal] when p.User or p.Tool is blank or "unknown".
//   - [ErrPrincipalLookup] when the lookup failed or was ambiguous.
//   - any other error when the membership walk failed. A partial closure is
//     never returned as complete.
func (r *IdentityResolver) Resolve(ctx context.Context, p principal.Principal) (Identity, error) {
	if isUnstamped(p) {
		return Identity{}, fmt.Errorf("%w: User=%q Tool=%q", ErrUnstampedPrincipal, p.User, p.Tool)
	}
	unmatched := false
	if r.policy.principalPropertyLookupEnabled() {
		id, err := r.ResolvePrincipal(ctx, p.User)
		if err != nil {
			return Identity{}, fmt.Errorf("%w: %w", ErrPrincipalLookup, err)
		}
		switch {
		case id == "":
			unmatched = true
		case id != p.User:
			p = substituteUser(p, id)
		}
	}
	ident, err := r.ResolveDirect(ctx, p)
	if err != nil {
		return Identity{}, err
	}
	ident.unmatched = unmatched
	return ident, nil
}

// substituteUser returns p with User replaced by the resolved entity id and
// RawUser set to the original identifier. It rebuilds through
// [principal.VerifiedFrom] so every assertion claim survives. A plain
// composite literal would drop the roles and, worse, the attenuation claims,
// which would silently widen a restricted client to its user's full grants.
func substituteUser(p principal.Principal, id string) principal.Principal {
	out := principal.VerifiedFrom(id, p.Tool, principal.Claims{
		OrgID:         p.OrgID(),
		OrgSlug:       p.OrgSlug(),
		OrgName:       p.OrgName(),
		Roles:         p.Roles(),
		PrincipalType: p.PrincipalType(),
		Scopes:        p.Scopes(),
		Email:         p.Email(),
	})
	out.RawUser = p.User
	return out
}

// ResolveDirect resolves p without the `principal_property` lookup: p.User is
// taken as the graph identity as it stands. The CLI, MCP, the scheduler and
// Lua use it, because their principals are not external natural keys. The
// data-entry edge also uses it to refresh an identity it already resolved.
//
// It returns [ErrUnstampedPrincipal] for an unstamped principal and the
// backing error when the membership walk fails.
func (r *IdentityResolver) ResolveDirect(ctx context.Context, p principal.Principal) (Identity, error) {
	if isUnstamped(p) {
		return Identity{}, fmt.Errorf("%w: User=%q Tool=%q", ErrUnstampedPrincipal, p.User, p.Tool)
	}
	members, err := r.walkMembers(ctx, p.User)
	if err != nil {
		return Identity{}, fmt.Errorf("acl: resolve membership of %q: %w", p.User, err)
	}
	return Identity{principal: p.Clone(), members: members, origin: r}, nil
}

// ResolvePrincipal maps a raw principal identifier to a user entity id via the
// policy's `principal_property` lookup. It returns:
//
//   - ("", nil)  when the lookup is disabled, rawUser is blank, or no entity
//     matches.
//   - (id, nil)  on exactly one match.
//   - ("", err)  when the lookup errored or more than one entity matched (an
//     ambiguous natural key, a data-integrity failure it refuses to guess
//     through).
//
// [IdentityResolver.Resolve] uses it. It is exported for callers that need the
// id without an Identity: the `current_user` predicate functions and the
// aclmap principal mapping.
//
//archplan:guard=authn reason="maps the identity from the JWT gate or the trusted proxy header to a person entity; verifies nothing itself"
func (r *IdentityResolver) ResolvePrincipal(ctx context.Context, rawUser string) (string, error) {
	if !r.policy.principalPropertyLookupEnabled() || strings.TrimSpace(rawUser) == "" {
		return "", nil
	}
	if r.lookup == nil {
		// Defense in depth: NewDeclarative rejects this combination, but a
		// future direct construction path must not silently over-resolve.
		return "", errors.New("acl: ResolvePrincipal: lookup enabled but no PrincipalLookup configured")
	}
	ids, err := r.lookup.LookupEntityByProperty(
		ctx, r.policy.UserEntityType, r.policy.PrincipalProperty, rawUser)
	if err != nil {
		return "", err
	}
	switch len(ids) {
	case 0:
		return "", nil
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("acl: principal %q matches %d %s entities on %q (ambiguous natural key)",
			rawUser, len(ids), r.policy.UserEntityType, r.policy.PrincipalProperty)
	}
}

// walkMembers returns {user} ∪ the transitive closure over the configured
// membership relation (default `member-of`, see
// [Policy.EffectiveMembershipRelation]). A visited set terminates cycles;
// depthCap is the backstop. A graph error aborts the walk and is returned:
// a partial closure would silently drop group grants.
func (r *IdentityResolver) walkMembers(ctx context.Context, user string) ([]string, error) {
	if user == "" {
		return nil, nil
	}
	rel := r.policy.EffectiveMembershipRelation()
	visited := map[string]bool{user: true}
	order := []string{user}
	frontier := []string{user}
	for depth := 0; depth < depthCap && len(frontier) > 0; depth++ {
		var next []string
		for _, n := range frontier {
			tos, err := r.members.OutgoingRelations(ctx, n, rel)
			if err != nil {
				return nil, err
			}
			for _, to := range tos {
				if visited[to] {
					continue
				}
				visited[to] = true
				order = append(order, to)
				next = append(next, to)
			}
		}
		frontier = next
	}
	return order, nil
}

// OpenRequest is the request-edge step that turns a principal into a
// [Request] on d. It resolves p with [IdentityResolver.ResolveDirect], so
// p.User must already be the graph identity, and opens the Request with
// [Declarative.ForIdentity].
//
// It is a package-level function, not a method, on purpose: evaluator
// methods may not resolve identities (TestEvaluatorDoesNotResolveIdentity),
// and only the edge files listed in TestIdentityEdgeCallers may call it.
//
// Nil: rejected — d must be a non-nil *Declarative; it panics otherwise,
// because a nil d is a wiring bug.
func OpenRequest(ctx context.Context, d *Declarative, p principal.Principal) (*Request, error) {
	ident, err := d.IdentityResolver().ResolveDirect(ctx, p)
	if err != nil {
		return nil, err
	}
	return d.ForIdentity(ident)
}

// BindRequest is the request-edge step for transports whose principal is
// already a graph identity (CLI, MCP, scheduler, Lua). It opens a [Request]
// for the principal on ctx with [OpenRequest] and attaches it with
// [WithRequest].
//
// For any ACL other than a non-nil *Declarative it returns ctx unchanged:
// [NopACL] and [ReadOnlyACL] evaluate nothing per principal, and a typed-nil
// *Declarative means no policy is configured.
//
// Call it once per operation, after the principal is stamped. Evaluation paths
// read the attached Request and fail closed without one.
func BindRequest(ctx context.Context, a ACL) (context.Context, error) {
	d, ok := a.(*Declarative)
	if !ok || d == nil {
		return ctx, nil
	}
	req, err := OpenRequest(ctx, d, principal.From(ctx))
	if err != nil {
		return ctx, err
	}
	return WithRequest(ctx, req), nil
}
