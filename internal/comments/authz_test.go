package comments_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/comments"
)

// fakeGate grants exactly the permissions it is given, for every subject.
type fakeGate struct {
	granted map[string]bool
	// subjectScoped, when set, grants only for this subject id — used to prove
	// the gate is asked about the TARGET entity, not some ambient subject.
	subjectScoped string
	asked         []string
}

func (g *fakeGate) HoldsPermission(_ context.Context, subjectID, permission string) bool {
	g.asked = append(g.asked, subjectID+"/"+permission)
	if g.subjectScoped != "" && subjectID != g.subjectScoped {
		return false
	}
	return g.granted[permission]
}

type fakeReads struct {
	allow bool
	asked []string
}

func (r *fakeReads) CanRead(_ context.Context, entityType, entityID string) bool {
	r.asked = append(r.asked, entityType+"/"+entityID)
	return r.allow
}

func grants(perms ...string) *fakeGate {
	m := map[string]bool{}
	for _, p := range perms {
		m[p] = true
	}
	return &fakeGate{granted: m}
}

var tgt = comments.Target{Type: "ticket", ID: "TKT-1"}

func aliceComment() comments.Comment {
	return comments.Comment{ID: "c1", Author: "alice@example.com", Body: "x"}
}

// TestAuthorizer_PermissionStringsMatchACL pins the duplicated permission
// names against the ACL's constants. This package cannot import internal/acl
// (it sits below the authorization layer), so the strings are copied — and a
// silent divergence would mean a permission an operator grants is never the one
// checked, which no other test would catch.
func TestAuthorizer_PermissionStringsMatchACL(t *testing.T) {
	perms := grants(acl.PermCommentRead)
	reads := &fakeReads{allow: true}
	a := comments.NewAuthorizer(perms, reads, true)

	require.True(t, a.CanRead(context.Background(), tgt),
		"acl.PermCommentRead must be the string the authorizer asks for")

	for _, tc := range []struct {
		name string
		perm string
		call func(*comments.Authorizer) bool
	}{
		{"add", acl.PermCommentAdd, func(a *comments.Authorizer) bool {
			return a.CanAdd(context.Background(), tgt)
		}},
		{"update-any", acl.PermCommentUpdateAny, func(a *comments.Authorizer) bool {
			return a.CanUpdate(context.Background(), tgt, aliceComment(), "bob@example.com")
		}},
		{"delete-any", acl.PermCommentDeleteAny, func(a *comments.Authorizer) bool {
			return a.CanDelete(context.Background(), tgt, aliceComment(), "bob@example.com")
		}},
		{"update-own", acl.PermCommentUpdateOwn, func(a *comments.Authorizer) bool {
			return a.CanUpdate(context.Background(), tgt, aliceComment(), "alice@example.com")
		}},
		{"delete-own", acl.PermCommentDeleteOwn, func(a *comments.Authorizer) bool {
			return a.CanDelete(context.Background(), tgt, aliceComment(), "alice@example.com")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := comments.NewAuthorizer(grants(tc.perm), &fakeReads{allow: true}, true)
			require.True(t, tc.call(auth), "%s must be checked as %q", tc.name, tc.perm)
		})
	}
}

// TestAuthorizer_InertWithoutPolicy pins the CLI/desktop tier: with no policy
// there is no principal to authorize, so everything is permitted.
func TestAuthorizer_InertWithoutPolicy(t *testing.T) {
	a := comments.NewAuthorizer(nil, nil, false)
	ctx := context.Background()

	require.True(t, a.CanRead(ctx, tgt))
	require.True(t, a.CanAdd(ctx, tgt))
	require.True(t, a.CanUpdate(ctx, tgt, aliceComment(), ""))
	require.True(t, a.CanDelete(ctx, tgt, aliceComment(), ""))
}

// TestAuthorizer_FailsClosedWhenGatesMissing pins the other half of that rule:
// a policy IS configured but the gate is absent, which is a wiring failure on a
// served path. Opening commenting because plumbing broke would be the wrong
// direction to fail.
func TestAuthorizer_FailsClosedWhenGatesMissing(t *testing.T) {
	ctx := context.Background()

	t.Run("no permission gate", func(t *testing.T) {
		a := comments.NewAuthorizer(nil, &fakeReads{allow: true}, true)
		require.False(t, a.CanRead(ctx, tgt))
		require.False(t, a.CanAdd(ctx, tgt))
	})

	t.Run("no read gate", func(t *testing.T) {
		a := comments.NewAuthorizer(grants(acl.PermCommentRead), nil, true)
		require.False(t, a.CanRead(ctx, tgt))
	})
}

// TestAuthorizer_ReadFloor pins that comment grants cannot override the
// target's read verdict (AC7). Otherwise a principal granted comment:read
// globally could probe which entities exist by asking for their comments.
func TestAuthorizer_ReadFloor(t *testing.T) {
	ctx := context.Background()
	denied := &fakeReads{allow: false}
	a := comments.NewAuthorizer(
		grants(acl.PermCommentRead, acl.PermCommentAdd,
			acl.PermCommentUpdateAny, acl.PermCommentDeleteAny),
		denied, true)

	require.False(t, a.CanRead(ctx, tgt), "unreadable target: no comment read")
	require.False(t, a.CanAdd(ctx, tgt), "unreadable target: no comment add")
	require.False(t, a.CanUpdate(ctx, tgt, aliceComment(), "alice@example.com"))
	require.False(t, a.CanDelete(ctx, tgt, aliceComment(), "alice@example.com"))
}

// TestAuthorizer_VerbsAreIndependent pins AC5: holding one comment permission
// does not confer another.
func TestAuthorizer_VerbsAreIndependent(t *testing.T) {
	ctx := context.Background()

	t.Run("read does not confer add", func(t *testing.T) {
		a := comments.NewAuthorizer(grants(acl.PermCommentRead), &fakeReads{allow: true}, true)
		require.True(t, a.CanRead(ctx, tgt))
		require.False(t, a.CanAdd(ctx, tgt))
	})

	t.Run("add does not confer read", func(t *testing.T) {
		// Write-only commenting is a deliberate posture, so this direction
		// must work too.
		a := comments.NewAuthorizer(grants(acl.PermCommentAdd), &fakeReads{allow: true}, true)
		require.True(t, a.CanAdd(ctx, tgt))
		require.False(t, a.CanRead(ctx, tgt))
	})

	t.Run("update does not confer delete", func(t *testing.T) {
		a := comments.NewAuthorizer(grants(acl.PermCommentUpdateAny), &fakeReads{allow: true}, true)
		c := aliceComment()
		require.True(t, a.CanUpdate(ctx, tgt, c, "bob@example.com"))
		require.False(t, a.CanDelete(ctx, tgt, c, "bob@example.com"))
	})
}

// TestAuthorizer_OwnVersusAny pins the ownership split.
func TestAuthorizer_OwnVersusAny(t *testing.T) {
	ctx := context.Background()
	reads := func() *fakeReads { return &fakeReads{allow: true} }
	alice := aliceComment()

	t.Run("own permits editing your own comment", func(t *testing.T) {
		a := comments.NewAuthorizer(grants(acl.PermCommentUpdateOwn), reads(), true)
		require.True(t, a.CanUpdate(ctx, tgt, alice, "alice@example.com"))
	})

	t.Run("own refuses someone else's comment", func(t *testing.T) {
		a := comments.NewAuthorizer(grants(acl.PermCommentUpdateOwn), reads(), true)
		require.False(t, a.CanUpdate(ctx, tgt, alice, "bob@example.com"))
	})

	t.Run("any permits someone else's comment", func(t *testing.T) {
		a := comments.NewAuthorizer(grants(acl.PermCommentUpdateAny), reads(), true)
		require.True(t, a.CanUpdate(ctx, tgt, alice, "bob@example.com"))
	})

	t.Run("any implies own", func(t *testing.T) {
		// A moderator holding only -any must not need -own as well to edit
		// their own comment.
		a := comments.NewAuthorizer(grants(acl.PermCommentDeleteAny), reads(), true)
		require.True(t, a.CanDelete(ctx, tgt, alice, "alice@example.com"))
	})

	t.Run("empty author never establishes ownership", func(t *testing.T) {
		// Defense against a hand-edited file: an author-less comment must not
		// become editable by an author-less principal.
		a := comments.NewAuthorizer(grants(acl.PermCommentUpdateOwn), reads(), true)
		require.False(t, a.CanUpdate(ctx, tgt, comments.Comment{ID: "c1"}, ""))
	})
}

// TestAuthorizer_AsksAboutTheTargetEntity pins that permissions are resolved
// per target, which is what lets a role conferred by an ownership relation to
// that entity grant them (AC6).
func TestAuthorizer_AsksAboutTheTargetEntity(t *testing.T) {
	ctx := context.Background()
	perms := grants(acl.PermCommentRead)
	perms.subjectScoped = "TKT-1"
	reads := &fakeReads{allow: true}
	a := comments.NewAuthorizer(perms, reads, true)

	require.True(t, a.CanRead(ctx, comments.Target{Type: "ticket", ID: "TKT-1"}))
	require.False(t, a.CanRead(ctx, comments.Target{Type: "ticket", ID: "TKT-2"}),
		"a grant scoped to one entity must not leak to another")

	require.Contains(t, reads.asked, "ticket/TKT-1")
	require.Contains(t, perms.asked, "TKT-1/comment:read")
}
