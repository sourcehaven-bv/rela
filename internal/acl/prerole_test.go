package acl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// preRolePolicy has the principal lookup configured, so a verified principal
// with no user entity is unmatched, plus a client ceiling for apps.
const preRolePolicy = `
user_entity_type: person
principal_property: sub
roles:
  editor: {read: [requirement], create: [requirement], update: [requirement], convert: [requirement]}
  reader: {read: [requirement]}
assignments:
  alice: editor
  bob: reader
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
scope_grants:
  rela.req.write:
    update: [requirement]
`

func preRoleWorld(t *testing.T, unmatched string) *acl.Declarative {
	t.Helper()
	p, err := acl.LoadPolicyBytes([]byte(preRolePolicy + "unmatched_principal: " + unmatched + "\n"))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	st := memstore.New()
	d, err := acl.NewDeclarative(p, aclstore.MustNewGraph(st), st,
		acl.WithPrincipalLookup(aclstore.MustNewPrincipalLookup(st)))
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

func asUser(ctx context.Context, user, principalType string, scopes ...string) context.Context {
	return principal.With(ctx, principal.VerifiedFrom(user, principal.ToolDataEntry,
		principal.Claims{PrincipalType: principalType, Scopes: scopes}))
}

// TestAuthorizePreRoleWrite_SharesChecksWithAuthorizeWrite pins that every
// check of Declarative.AuthorizeWrite that does not come from a role grant
// refuses an elevated write the same way (TKT-JA5SABG), and that the role
// grants are the only thing AuthorizePreRoleWrite leaves out. Both paths run
// the same functions (openWrite, entityPreRoleDenial), so a check added there
// reaches both; a new pre-role check belongs in this table.
func TestAuthorizePreRoleWrite_SharesChecksWithAuthorizeWrite(t *testing.T) {
	t.Parallel()
	update := acl.WriteRequest{Op: acl.OpUpdate, Subject: acl.NewFacelessEntitySubject("requirement", "REQ-1")}
	convert := acl.WriteRequest{Op: acl.OpConvert,
		Subject: acl.NewConversionSubject("requirement", "REQ-1", "requirement")}

	tests := []struct {
		name      string
		unmatched string
		req       acl.WriteRequest
		bind      func(t *testing.T, d *acl.Declarative) context.Context
		// wantKind is the shared denial; "" means AuthorizePreRoleWrite
		// allows and only the role grants decide AuthorizeWrite.
		wantKind  string
		wantCause error
		roleAllow bool
	}{
		{
			name: "no request on ctx", unmatched: acl.UnmatchedAnonymous, req: update,
			bind: func(t *testing.T, _ *acl.Declarative) context.Context {
				t.Helper()
				return asUser(t.Context(), "alice", "")
			},
			wantKind: "role-grant", wantCause: acl.ErrNoIdentity,
		},
		{
			name: "request from another policy", unmatched: acl.UnmatchedAnonymous, req: update,
			bind: func(t *testing.T, _ *acl.Declarative) context.Context {
				t.Helper()
				return acltest.Bind(asUser(t.Context(), "alice", ""), t, preRoleWorld(t, acl.UnmatchedAnonymous))
			},
			wantKind: "role-grant", wantCause: acl.ErrForeignIdentity,
		},
		{
			name: "request bound to another principal", unmatched: acl.UnmatchedAnonymous, req: update,
			bind: func(t *testing.T, d *acl.Declarative) context.Context {
				t.Helper()
				return asUser(acltest.Bind(asUser(t.Context(), "alice", ""), t, d), "bob", "")
			},
			wantKind: "role-grant", wantCause: acl.ErrIdentityMismatch,
		},
		{
			name: "unmatched verified principal under reject", unmatched: acl.UnmatchedReject, req: update,
			bind: func(t *testing.T, d *acl.Declarative) context.Context {
				t.Helper()
				return acltest.BindVerified(asUser(t.Context(), "alice", ""), t, d)
			},
			wantKind: "unmatched-principal",
		},
		{
			name: "client ceiling withholds update", unmatched: acl.UnmatchedAnonymous, req: update,
			bind: func(t *testing.T, d *acl.Declarative) context.Context {
				t.Helper()
				return acltest.Bind(asUser(t.Context(), "alice", "app"), t, d)
			},
			wantKind: "client-ceiling",
		},
		{
			name: "client ceiling withholds a conversion", unmatched: acl.UnmatchedAnonymous, req: convert,
			bind: func(t *testing.T, d *acl.Declarative) context.Context {
				t.Helper()
				return acltest.Bind(asUser(t.Context(), "alice", "app"), t, d)
			},
			wantKind: "client-ceiling",
		},
		{
			name: "unmatched verified principal under anonymous", unmatched: acl.UnmatchedAnonymous, req: update,
			bind: func(t *testing.T, d *acl.Declarative) context.Context {
				t.Helper()
				return acltest.BindVerified(asUser(t.Context(), "alice", ""), t, d)
			},
			roleAllow: true,
		},
		{
			name: "scope grant re-opens update", unmatched: acl.UnmatchedAnonymous, req: update,
			bind: func(t *testing.T, d *acl.Declarative) context.Context {
				t.Helper()
				return acltest.Bind(asUser(t.Context(), "alice", "app", "rela.req.write"), t, d)
			},
			roleAllow: true,
		},
		{
			name: "role grant missing", unmatched: acl.UnmatchedAnonymous, req: update,
			bind: func(t *testing.T, d *acl.Declarative) context.Context {
				t.Helper()
				return acltest.Bind(asUser(t.Context(), "bob", ""), t, d)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := preRoleWorld(t, tc.unmatched)
			ctx := tc.bind(t, d)
			full := d.AuthorizeWrite(ctx, tc.req)
			pre := acl.AuthorizePreRoleWrite(ctx, d, tc.req)

			if tc.wantKind == "" {
				if !pre.Allow {
					t.Fatalf("pre-role denied: %s %s (%s)", pre.RuleKind, pre.RuleID, pre.Reason)
				}
				if full.Allow != tc.roleAllow {
					t.Fatalf("AuthorizeWrite allow = %v, want %v (only role grants differ): %s",
						full.Allow, tc.roleAllow, full.Reason)
				}
				return
			}
			for name, dec := range map[string]acl.Decision{"AuthorizeWrite": full, "AuthorizePreRoleWrite": pre} {
				if dec.Allow || dec.RuleKind != tc.wantKind {
					t.Errorf("%s = allow %v kind %q, want a %q denial", name, dec.Allow, dec.RuleKind, tc.wantKind)
				}
				if tc.wantCause != nil && !errors.Is(dec.Cause, tc.wantCause) {
					t.Errorf("%s cause = %v, want %v", name, dec.Cause, tc.wantCause)
				}
			}
			if full.RuleID != pre.RuleID || full.Reason != pre.Reason {
				t.Errorf("decisions differ: AuthorizeWrite %q/%q, AuthorizePreRoleWrite %q/%q",
					full.RuleID, full.Reason, pre.RuleID, pre.Reason)
			}
		})
	}
}

// TestAuthorizePreRoleWrite_OtherACLs pins the answer for ACLs without
// separable checks: read-only denies, no policy allows, and nil denies.
func TestAuthorizePreRoleWrite_OtherACLs(t *testing.T) {
	t.Parallel()
	req := acl.WriteRequest{Op: acl.OpUpdate, Subject: acl.NewFacelessEntitySubject("requirement", "REQ-1")}
	var nilDecl *acl.Declarative
	for _, tc := range []struct {
		name string
		a    acl.ACL
		want bool
	}{
		{"read-only", acl.ReadOnlyACL{}, false},
		{"no policy", acl.NopACL{}, true},
		{"nil", nil, false},
		{"nil declarative", nilDecl, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := acl.AuthorizePreRoleWrite(t.Context(), tc.a, req).Allow; got != tc.want {
				t.Errorf("allow = %v, want %v", got, tc.want)
			}
		})
	}
}
