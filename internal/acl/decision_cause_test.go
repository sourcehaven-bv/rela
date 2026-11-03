package acl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestAuthorizeWrite_NoRequestScopeKeepsCause pins that a write denied for a
// missing or mismatched Request carries the sentinel as Decision.Cause, and
// that ForbiddenError unwraps to it. The data-entry edge maps ErrNoIdentity
// to 401 and a mismatch to 500 through errors.Is on that chain (BUG-O51NS5).
func TestAuthorizeWrite_NoRequestScopeKeepsCause(t *testing.T) {
	t.Parallel()
	d := newACL(t, testPolicy())
	cases := []struct {
		name string
		ctx  func() context.Context
		want error
	}{
		{name: "no request", ctx: func() context.Context { return ctxAs("alice") }, want: acl.ErrNoIdentity},
		{
			name: "request for another principal",
			ctx: func() context.Context {
				return principal.With(ctxFor(t, d, "bob"),
					principal.Principal{User: "alice", Tool: principal.ToolCLI})
			},
			want: acl.ErrIdentityMismatch,
		},
		{
			name: "request from another policy",
			ctx:  func() context.Context { return ctxFor(t, newACL(t, testPolicy()), "alice") },
			want: acl.ErrForeignIdentity,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dec := d.AuthorizeWrite(tc.ctx(), entityCreate("ticket"))
			if dec.Allow {
				t.Fatal("AuthorizeWrite allowed a write with no usable request")
			}
			if !errors.Is(dec.Cause, tc.want) {
				t.Fatalf("Decision.Cause = %v, want %v", dec.Cause, tc.want)
			}
			err := error(&acl.ForbiddenError{Decision: dec})
			if !errors.Is(err, tc.want) || !errors.Is(err, acl.ErrForbidden) {
				t.Fatalf("ForbiddenError chain: Is(%v)=%v Is(ErrForbidden)=%v",
					tc.want, errors.Is(err, tc.want), errors.Is(err, acl.ErrForbidden))
			}
		})
	}

	t.Run("rule deny has no cause", func(t *testing.T) {
		t.Parallel()
		dec := d.AuthorizeWrite(ctxFor(t, d, "nobody"), entityCreate("ticket"))
		if dec.Allow {
			t.Fatal("nobody may not create a ticket under testPolicy")
		}
		if dec.Cause != nil {
			t.Fatalf("Decision.Cause = %v on a rule deny, want nil", dec.Cause)
		}
		if errors.Is(&acl.ForbiddenError{Decision: dec}, acl.ErrNoIdentity) {
			t.Fatal("a rule deny must not read as ErrNoIdentity")
		}
	})
}
