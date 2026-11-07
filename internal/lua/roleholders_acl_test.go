package lua_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// typedHolders returns fixed holder rows of one entity type.
type typedHolders struct {
	userType string
	ids      []string
}

func (h typedHolders) PrincipalsWithRole(context.Context, string) (string, []entity.Ref, error) {
	refs := make([]entity.Ref, 0, len(h.ids))
	for _, id := range h.ids {
		refs = append(refs, entity.Ref{ID: id, Face: entity.ImplicitFace})
	}
	return h.userType, refs, nil
}

// TestScriptReads_PrincipalsWithRoleGated (TKT-IZL6NX8) runs
// rela.principals_with_role through the real declarative read gate: a holder
// the caller may read is returned, and one the policy hides is not.
func TestScriptReads_PrincipalsWithRoleGated(t *testing.T) {
	tests := []struct {
		name    string
		holders typedHolders
		want    string
	}{
		{name: "readable holder", holders: typedHolders{userType: "person", ids: []string{"P-1"}}, want: "P-1"},
		{name: "hidden holder", holders: typedHolders{userType: "secret", ids: []string{"SEC-1"}}, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, deps, decl := newACLWorld(t)
			deps.RoleHolders = tc.holders
			out := runAsAlice(t, decl, deps, `rela.output(table.concat(rela.principals_with_role("cpo"), ","))`)
			// rela.output writes a string as a quoted value.
			if got := strings.Trim(strings.TrimSpace(out), `"`); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
