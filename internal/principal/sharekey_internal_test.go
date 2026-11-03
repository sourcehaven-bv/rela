package principal

import "testing"

// ShareKey agrees with Equal: each variant differs from base in one field
// Equal compares, and only the nil-versus-empty slice case is Equal.
func TestShareKey_MatchesEqual(t *testing.T) {
	base := Principal{
		User: "alice", Tool: ToolDataEntry, RawUser: "a", orgID: "o", orgSlug: "s", orgName: "n",
		principalType: "user", email: "a@x", roles: []string{"r"}, scopes: []string{"read"},
	}
	with := func(f func(*Principal)) Principal {
		p := base.Clone()
		f(&p)
		return p
	}
	cases := []struct {
		name string
		p    Principal
	}{
		{"same", base.Clone()},
		{"user", with(func(p *Principal) { p.User = "bob" })},
		{"tool", with(func(p *Principal) { p.Tool = ToolCLI })},
		{"raw user", with(func(p *Principal) { p.RawUser = "b" })},
		{"org id", with(func(p *Principal) { p.orgID = "o2" })},
		{"org slug", with(func(p *Principal) { p.orgSlug = "s2" })},
		{"org name", with(func(p *Principal) { p.orgName = "n2" })},
		{"principal type", with(func(p *Principal) { p.principalType = "agent" })},
		{"email", with(func(p *Principal) { p.email = "b@x" })},
		{"roles", with(func(p *Principal) { p.roles = []string{"r", "s"} })},
		{"scopes", with(func(p *Principal) { p.scopes = nil })},
		{"field shift", with(func(p *Principal) { p.User, p.Tool = "alice"+ToolDataEntry, "" })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := tc.p.ShareKey() == base.ShareKey(), tc.p.Equal(base); got != want {
				t.Fatalf("ShareKey equal = %v, Equal = %v", got, want)
			}
		})
	}
	empty := Principal{User: "alice", roles: []string{}}
	if !empty.Equal(Principal{User: "alice"}) || empty.ShareKey() != (Principal{User: "alice"}).ShareKey() {
		t.Fatal("nil and empty roles must agree under Equal and ShareKey")
	}
}

func TestSameIdentity(t *testing.T) {
	a := Principal{User: "alice", Tool: ToolDataEntry, orgID: "o", roles: []string{"r"}}
	refreshed := a.Clone()
	refreshed.roles = []string{"admin"}
	refreshed.email = "new@x"
	if !a.SameIdentity(refreshed) {
		t.Fatal("changed claims made a different identity")
	}
	for _, other := range []Principal{
		{User: "bob", Tool: ToolDataEntry, orgID: "o"},
		{User: "alice", Tool: ToolCLI, orgID: "o"},
		{User: "alice", Tool: ToolDataEntry, orgID: "o2"},
		{User: "alice", Tool: ToolDataEntry, orgID: "o", RawUser: "x"},
	} {
		if a.SameIdentity(other) {
			t.Errorf("SameIdentity(%+v) = true", other)
		}
	}
}
