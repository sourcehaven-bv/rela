package access

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ErrPrincipalMismatch reports that ctx already carries an ACL request for a
// different principal than the one stamped on ctx. It is a wiring bug: the
// gate would run against the wrong identity, so the request must fail.
var ErrPrincipalMismatch = errors.New("access: attached ACL request belongs to another principal")

// ErrUnresolvedPrincipal reports that no ACL request could be opened for the
// principal on ctx, for example because the principal stamper left it
// unstamped. The cause is wrapped for the server log; it must not reach a
// response body (RR-372L).
var ErrUnresolvedPrincipal = errors.New("access: principal could not be resolved under the ACL")

// Opener opens the per-request access state under a declarative policy. One
// Opener serves every request of a router; [Opener.Open] runs once per HTTP
// request, so the member-of walk the acl.Request caches is shared by every
// gate call in that request (RR-JJYW).
type Opener struct {
	policy           *acl.Declarative
	verifiedIdentity bool
}

// NewOpener returns an Opener for policy. verifiedIdentity reports whether
// identity comes from a verified-JWT assertion (the gate is installed); only
// then can a principal that resolves to no user entity be marked
// unmatched-verified, which `unmatched_principal: reject` keys on.
//
// Nil: rejected. Without a policy there is no request to open; NopACL and
// ReadOnlyACL deployments do not use an Opener.
func NewOpener(policy *acl.Declarative, verifiedIdentity bool) (*Opener, error) {
	if policy == nil {
		return nil, errors.New("access: NewOpener: policy is nil")
	}
	return &Opener{policy: policy, verifiedIdentity: verifiedIdentity}, nil
}

// Open returns ctx with the ACL request for its principal attached, and that
// request. log receives the warnings; the caller scopes it to the HTTP request
// (path and method).
//
// Nil log: accepted — Open logs to slog.Default().
//
// When ctx already carries an ACL request (an upstream layer, chiefly tests,
// attached one), Open reuses it after checking that it belongs to the ctx
// principal; a mismatch returns [ErrPrincipalMismatch] (SIG-2). Reusing
// without the check would run every gate against the wrong identity with no
// loud signal. On this branch principal resolution has not run, so both sides
// are the raw principal.
//
// Otherwise Open resolves the principal through the policy's
// `principal_property` lookup and opens a request for the result. A failure
// returns [ErrUnresolvedPrincipal] rather than a ctx without a request, which
// would make every later read fail open (RR-875A).
func (o *Opener) Open(ctx context.Context, log *slog.Logger) (context.Context, *acl.Request, error) {
	if log == nil {
		log = slog.Default()
	}
	// The log messages keep the attachACLRequest prefix they had in dataentry,
	// so operators' existing searches still match.
	if existing := acl.FromContext(ctx); existing != nil {
		ctxPrin := principal.From(ctx)
		if !existing.Principal().Equal(ctxPrin) {
			log.Warn("acl: attachACLRequest: existing Request principal mismatch",
				"ctx_user", ctxPrin.User,
				"ctx_tool", ctxPrin.Tool,
				"req_user", existing.Principal().User,
				"req_tool", existing.Principal().Tool)
			return ctx, nil, ErrPrincipalMismatch
		}
		return ctx, existing, nil
	}

	ctx = o.resolvePrincipal(ctx, log)

	p := principal.From(ctx)
	req, err := o.policy.ForPrincipal(p)
	if err != nil {
		log.Warn("acl: attachACLRequest: ForPrincipal failed",
			"err", err,
			"user", p.User,
			"tool", p.Tool)
		return ctx, nil, errors.Join(ErrUnresolvedPrincipal, err)
	}
	return acl.WithRequest(ctx, req), req, nil
}

// Restamp re-stamps ctx to the user entity that sub now resolves to, after
// `unmatched_principal: provision` created it, and opens the ACL request for
// that principal. The returned ctx carries the resolved principal with RawUser
// set to sub, the matched-verified mark (so the triggering write is judged on
// the resolved identity's roles, not caught again by the unmatched gate) and
// the request.
//
// An ambiguous or failed re-resolve (two stubs raced onto one sub on a backend
// without atomic uniqueness) returns [ErrUnresolvedPrincipal], as does a
// failure to open the request. The caller then proceeds as unmatched with its
// original ctx rather than guess an identity.
//
// Restamp is a function rather than an [Opener] method because provisioning
// runs once per new user, outside the per-request lifecycle, and needs none
// of the Opener's settings.
//
// Nil policy: rejected with an error; there is no policy to resolve against.
func Restamp(ctx context.Context, policy *acl.Declarative, sub string) (context.Context, *acl.Request, error) {
	if policy == nil {
		return ctx, nil, errors.New("access: Restamp: policy is nil")
	}
	id, err := policy.ResolvePrincipal(ctx, sub)
	if err != nil {
		return ctx, nil, errors.Join(ErrUnresolvedPrincipal, err)
	}
	if id == "" {
		return ctx, nil, fmt.Errorf("%w: %q resolves to no single entity", ErrUnresolvedPrincipal, sub)
	}

	p := principal.From(ctx)
	// BUG-AR6NN9: principal.Verified keeps no PrincipalType or Scopes, so a
	// restricted client loses its ceiling here; Open keeps every claim through
	// VerifiedFrom. Kept as is in this refactor.
	resolved := principal.Verified(id, p.Tool, p.OrgID(), p.OrgSlug(), p.Roles()).WithEmail(p.Email())
	resolved.RawUser = sub
	restamped := acl.WithMatchedVerified(principal.With(ctx, resolved))

	req, err := policy.ForPrincipal(resolved)
	if err != nil {
		return ctx, nil, errors.Join(ErrUnresolvedPrincipal, err)
	}
	return acl.WithRequest(restamped, req), req, nil
}

// resolvePrincipal applies the policy's `principal_property` lookup to the ctx
// principal. When the lookup is enabled and the raw principal resolves to
// exactly one user entity, it returns a ctx carrying a principal whose User is
// the entity ID and whose RawUser is the original identifier, so the ACL walk
// and the audit log both attribute to the real entity. In every other case
// (lookup disabled, no match, ambiguous match, backend error) it returns ctx
// unchanged, falling back to the raw principal; errors are logged, a plain
// no-match is not (a break-glass identity assigned by raw UPN is expected to
// be absent from the graph).
//
// A genuine no-match on a verified identity while the lookup is enabled marks
// the request unmatched-verified, so `unmatched_principal: reject` can deny
// its writes (see acl.WithUnmatchedVerified). A disabled lookup also returns
// "", but that is "not attempted", not a no-match.
func (o *Opener) resolvePrincipal(ctx context.Context, log *slog.Logger) context.Context {
	p := principal.From(ctx)
	id, err := o.policy.ResolvePrincipal(ctx, p.User)
	if err != nil {
		log.Warn("acl: principal_property lookup failed; using raw principal", "err", err)
		return ctx
	}
	if id == "" || id == p.User {
		if o.verifiedIdentity && id == "" && o.policy.Policy().PrincipalPropertyLookupEnabled() {
			ctx = acl.WithUnmatchedVerified(ctx)
		}
		return ctx
	}
	// Rebuild via VerifiedFrom so the assertion claims survive the substitution.
	// A plain composite literal would drop org, roles, email and the
	// attenuation claims: the resolved principal would keep its identity but
	// lose every asserted grant and its ceiling. Losing the ceiling would
	// widen a restricted client to its acting user's full grants.
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
	return principal.With(ctx, out)
}
