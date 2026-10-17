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
// different principal (or another policy) than the one stamped on ctx. It is
// a wiring bug: the gate would run against the wrong identity, so the request
// must fail.
var ErrPrincipalMismatch = errors.New("access: attached ACL request belongs to another principal")

// ErrUnresolvedPrincipal reports that no ACL request could be opened for the
// principal on ctx, for example because the principal stamper left it
// unstamped or the membership walk failed. The cause is wrapped for the server
// log; it must not reach a response body (RR-372L).
var ErrUnresolvedPrincipal = errors.New("access: principal could not be resolved under the ACL")

// Opener opens the per-request access state under a declarative policy. One
// Opener serves every request of a router; [Opener.Open] runs once per HTTP
// request and resolves the identity once, so every gate call in that request
// evaluates the same membership closure (RR-JJYW).
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
// attached one), Open reuses it after [acl.Declarative.RequestFrom] checks
// that it belongs to this policy and to the ctx principal; a mismatch returns
// [ErrPrincipalMismatch] (SIG-2). Reusing without the check would run every
// gate against the wrong identity with no loud signal. On this branch
// principal resolution has not run, so both sides are the raw principal.
//
// Otherwise Open resolves the identity: the policy's `principal_property`
// lookup and the member-of walk. On a single match ctx is re-stamped with the
// resolved principal, so the ACL evaluation and the audit writer (which
// re-reads principal.From(ctx)) both attribute to the real entity, with the
// raw value in RawUser. A failure returns [ErrUnresolvedPrincipal] wrapping
// the cause, rather than a ctx without a request, which would make every
// later read fail open (RR-875A).
func (o *Opener) Open(ctx context.Context, log *slog.Logger) (context.Context, *acl.Request, error) {
	if log == nil {
		log = slog.Default()
	}
	// The log messages keep the attachACLRequest prefix they had in dataentry,
	// so operators' existing searches still match.
	existing, err := o.policy.RequestFrom(ctx)
	switch {
	case err == nil:
		return ctx, existing, nil
	case !errors.Is(err, acl.ErrNoIdentity):
		// RequestFrom logged the principal ids.
		log.Warn("acl: attachACLRequest: existing Request principal mismatch", "err", err)
		return ctx, nil, ErrPrincipalMismatch
	}

	ctx, ident, err := o.resolveIdentity(ctx, log)
	if err != nil {
		p := principal.From(ctx)
		log.Warn("acl: attachACLRequest: identity resolution failed",
			"err", err,
			"user", p.User,
			"tool", p.Tool)
		return ctx, nil, errors.Join(ErrUnresolvedPrincipal, err)
	}
	req, err := o.policy.ForIdentity(ident)
	if err != nil { // coverage-ignore: ident was just resolved by this policy's own resolver
		return ctx, nil, errors.Join(ErrUnresolvedPrincipal, err)
	}
	return acl.WithRequest(ctx, req), req, nil
}

// Restamp re-stamps ctx to the user entity its principal now resolves to,
// after `unmatched_principal: provision` created it, and opens the ACL request
// for that identity. The returned ctx carries the resolved principal with
// RawUser set to the verified subject, and the request. The request is opened
// on a matched identity, so it is not unmatched-verified: the triggering write
// is judged on the resolved identity's roles, not caught again by the
// unmatched gate.
//
// Resolution rebuilds the principal with every verified claim, so the client
// ceiling the original request carried still applies (BUG-AR6NN9).
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
func Restamp(ctx context.Context, policy *acl.Declarative) (context.Context, *acl.Request, error) {
	if policy == nil {
		return ctx, nil, errors.New("access: Restamp: policy is nil")
	}
	p := principal.From(ctx)
	ident, err := policy.IdentityResolver().Resolve(ctx, p)
	if err != nil {
		return ctx, nil, errors.Join(ErrUnresolvedPrincipal, err)
	}
	if ident.Unmatched() || !policy.Policy().PrincipalPropertyLookupEnabled() {
		return ctx, nil, fmt.Errorf("%w: %q resolves to no single entity", ErrUnresolvedPrincipal, p.User)
	}
	req, err := policy.ForIdentity(ident.WithVerifiedAssertion())
	if err != nil { // coverage-ignore: ident was just resolved by this policy's own resolver
		return ctx, nil, errors.Join(ErrUnresolvedPrincipal, err)
	}
	return acl.WithRequest(principal.With(ctx, ident.Principal()), req), req, nil
}

// resolveIdentity resolves the ctx principal into an [acl.Identity] for the
// request and returns ctx re-stamped with the identity's principal.
//
// With the policy's `principal_property` lookup enabled, a single match
// replaces User with the entity ID and keeps the raw value in RawUser;
// [acl.IdentityResolver.Resolve] rebuilds the principal so every verified
// claim survives (losing the ceiling claims would widen a restricted client;
// pinned by acl.TestResolve_SubstitutesAndKeepsClaims).
//
// A failed or ambiguous lookup is logged and resolution falls back to the raw
// principal ([acl.IdentityResolver.ResolveDirect]), the pre-feature behavior.
// That can only lose grants. A plain no-match is not logged: a principal
// absent from the graph is expected (a break-glass identity assigned by raw
// UPN).
//
// On a verified identity the result is marked with
// [acl.Identity.WithVerifiedAssertion]; combined with a genuine no-match the
// request opened on it is unmatched-verified, so `unmatched_principal:
// reject` can deny its writes (see [acl.Request.UnmatchedVerified]).
//
// Any other error (unstamped principal, failed membership walk) is returned.
func (o *Opener) resolveIdentity(
	ctx context.Context, log *slog.Logger,
) (context.Context, acl.Identity, error) {
	res := o.policy.IdentityResolver()
	p := principal.From(ctx)
	ident, err := res.Resolve(ctx, p)
	if errors.Is(err, acl.ErrPrincipalLookup) {
		log.Warn("acl: principal_property lookup failed; using raw principal", "err", err)
		ident, err = res.ResolveDirect(ctx, p)
	}
	if err != nil {
		return ctx, acl.Identity{}, err
	}
	if resolved := ident.Principal(); !resolved.Equal(p) {
		ctx = principal.With(ctx, resolved)
	}
	if o.verifiedIdentity {
		ident = ident.WithVerifiedAssertion()
	}
	return ctx, ident, nil
}
