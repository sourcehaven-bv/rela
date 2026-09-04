package acl

import (
	"strings"
	"testing"
)

// A client ceiling clamps by entity TYPE: permitsVerb matches a subject's bare
// type against the restriction lists. A `type@face` entry therefore never
// matches — in a denylist it removes nothing (fail-OPEN: `deny_update:
// [page@published]` left the face writable), in an allowlist it permits
// nothing. Neither can mean what the operator wrote, so both are refused at
// load, the same way `world:*` is refused on a role.
func TestRestriction_RefusesFaceShapedEntriesAndWorldWildcards(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		r    Restriction
		want string
	}{
		{"deny_update with a face", Restriction{DenyUpdate: []string{"page@published"}}, "names a face"},
		{"update with a face", Restriction{Update: []string{"page@published"}}, "names a face"},
		{"deny_write with a face", Restriction{DenyWrite: []string{"page@draft"}}, "names a face"},
		{"read with a face", Restriction{Read: []string{"page@published"}}, "names a face"},
		{"worlds wildcard", Restriction{Worlds: []string{"*"}}, "no wildcard"},
		{"deny_worlds wildcard", Restriction{DenyWorlds: []string{"*"}}, "no wildcard"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.r.validate("client_baselines.app")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want an error containing %q, got %v", tc.want, err)
			}
		})
	}

	// Positive control: the bare-type and named-world spellings still load.
	ok := Restriction{DenyUpdate: []string{"page"}, Worlds: []string{"published"}}
	if err := ok.validate("client_baselines.app"); err != nil {
		t.Errorf("a type-level clamp with a named world must validate; got %v", err)
	}
}
