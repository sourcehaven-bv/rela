// Package acltest holds test helpers for code that evaluates through an
// [acl.Declarative]. Production code resolves a principal once at the request
// edge; these helpers do the same for a test in one line.
package acltest

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// IdentityFor resolves p against d's store and policy without the
// `principal_property` lookup ([acl.IdentityResolver.ResolveDirect]) and fails
// the test on error.
func IdentityFor(ctx context.Context, tb testing.TB, d *acl.Declarative, p principal.Principal) acl.Identity {
	tb.Helper()
	id, err := d.IdentityResolver().ResolveDirect(ctx, p)
	if err != nil {
		tb.Fatalf("acltest.IdentityFor(%q): %v", p.User, err)
	}
	return id
}

// RequestFor opens a Request for p on d with [acl.OpenRequest] and fails the
// test on error.
func RequestFor(ctx context.Context, tb testing.TB, d *acl.Declarative, p principal.Principal) *acl.Request {
	tb.Helper()
	req, err := acl.OpenRequest(ctx, d, p)
	if err != nil {
		tb.Fatalf("acltest.RequestFor(%q): %v", p.User, err)
	}
	return req
}

// Bind does what a request edge does for the principal stamped on ctx: it
// resolves it, opens a Request on a, and attaches it. For an ACL that is not
// a *acl.Declarative it returns ctx unchanged. It fails the test on error.
func Bind(ctx context.Context, tb testing.TB, a acl.ACL) context.Context {
	tb.Helper()
	out, err := acl.BindRequest(ctx, a)
	if err != nil {
		tb.Fatalf("acltest.Bind(%q): %v", principal.From(ctx).User, err)
	}
	return out
}

// BindVerified does what the data-entry edge does behind a verified-JWT gate:
// it resolves the principal on ctx with the `principal_property` lookup
// ([acl.IdentityResolver.Resolve]), marks the identity as coming from a
// verified assertion, re-stamps ctx with the resolved principal, and attaches
// the Request. A principal the lookup does not match yields an
// unmatched-verified Request. It fails the test on error.
func BindVerified(ctx context.Context, tb testing.TB, d *acl.Declarative) context.Context {
	tb.Helper()
	id, err := d.IdentityResolver().Resolve(ctx, principal.From(ctx))
	if err != nil {
		tb.Fatalf("acltest.BindVerified(%q): %v", principal.From(ctx).User, err)
	}
	req, err := d.ForIdentity(id.WithVerifiedAssertion())
	if err != nil {
		tb.Fatalf("acltest.BindVerified(%q): %v", principal.From(ctx).User, err)
	}
	return acl.WithRequest(principal.With(ctx, id.Principal()), req)
}
