package acl_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// TestCommentPermissions_AreBuiltin pins that every comment permission is
// registered. An unregistered constant is reported as dead config by
// `rela acl audit` while it is in fact live — the exact failure that let
// history:read be flagged as a typo.
func TestCommentPermissions_AreBuiltin(t *testing.T) {
	t.Parallel()
	builtin := acl.BuiltinPermissions()
	for _, perm := range []string{
		acl.PermCommentRead,
		acl.PermCommentAdd,
		acl.PermCommentUpdateOwn,
		acl.PermCommentUpdateAny,
		acl.PermCommentDeleteOwn,
		acl.PermCommentDeleteAny,
	} {
		if !slices.Contains(builtin, perm) {
			t.Errorf("BuiltinPermissions() is missing %q", perm)
		}
	}
}

// TestCommentPermissions_MutatingRequiresRead pins the coherence rule
// (RR-60067I): a role that may change comments must be able to read them.
// Without it a policy granting only comment:delete-any loads cleanly and lets
// its holder remove comments it can never see.
func TestCommentPermissions_MutatingRequiresRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		permissions []string
		wantErr     bool
	}{
		{
			name:        "update-own without read is refused",
			permissions: []string{acl.PermCommentUpdateOwn},
			wantErr:     true,
		},
		{
			name:        "update-any without read is refused",
			permissions: []string{acl.PermCommentUpdateAny},
			wantErr:     true,
		},
		{
			name:        "delete-own without read is refused",
			permissions: []string{acl.PermCommentDeleteOwn},
			wantErr:     true,
		},
		{
			name:        "delete-any without read is refused",
			permissions: []string{acl.PermCommentDeleteAny},
			wantErr:     true,
		},
		{
			name:        "add without read is allowed (write-only commenting)",
			permissions: []string{acl.PermCommentAdd},
			wantErr:     false,
		},
		{
			name:        "read alone is allowed",
			permissions: []string{acl.PermCommentRead},
			wantErr:     false,
		},
		{
			name:        "mutating with read is allowed",
			permissions: []string{acl.PermCommentRead, acl.PermCommentDeleteAny},
			wantErr:     false,
		},
		{
			name:        "no comment permissions at all",
			permissions: nil,
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			b.WriteString("roles:\n  reviewer:\n    read: [\"*\"]\n")
			if len(tc.permissions) > 0 {
				b.WriteString("    permissions: [" + strings.Join(tc.permissions, ", ") + "]\n")
			}

			_, err := acl.LoadPolicyBytes([]byte(b.String()))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected a load error for %v, got none", tc.permissions)
				}
				// The message must name both the offending permission and the
				// fix, since an operator reading it has only the text to go on.
				if !strings.Contains(err.Error(), acl.PermCommentRead) {
					t.Errorf("error should name %q, got: %v", acl.PermCommentRead, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected load error: %v", err)
			}
		})
	}
}

// TestCommentPermissions_ReadOnAnotherRoleDoesNotCount pins that the coherence
// rule is evaluated PER ROLE. Roles are unioned at request time, but a policy
// is validated role by role: a reviewer role holding delete-any is misconfigured
// even if some other role in the file happens to grant read, because nothing
// guarantees the same principal holds both.
func TestCommentPermissions_ReadOnAnotherRoleDoesNotCount(t *testing.T) {
	t.Parallel()

	policy := `
roles:
  reader:
    read: ["*"]
    permissions: [comment:read]
  moderator:
    read: ["*"]
    permissions: [comment:delete-any]
`
	if _, err := acl.LoadPolicyBytes([]byte(policy)); err == nil {
		t.Fatal("expected a load error: moderator lacks comment:read of its own")
	}
}
