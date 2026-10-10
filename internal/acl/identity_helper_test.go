package acl

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// identityFor is the in-package twin of acltest.IdentityFor, which this
// package's internal tests cannot import (acltest imports acl).
func identityFor(ctx context.Context, tb testing.TB, d *Declarative, p principal.Principal) Identity {
	tb.Helper()
	id, err := d.IdentityResolver().ResolveDirect(ctx, p)
	if err != nil {
		tb.Fatalf("identityFor(%q): %v", p.User, err)
	}
	return id
}
