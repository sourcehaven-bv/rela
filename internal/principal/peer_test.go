package principal_test

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Peer principals are reserved and carry principal_type peer (DEC-81161C3).

func TestPeer_IdentityAndType(t *testing.T) {
	t.Parallel()
	p := principal.Peer("acme", principal.ToolDataEntry)
	if p.User != "peer:acme" {
		t.Errorf("User = %q, want peer:acme", p.User)
	}
	if p.Tool != principal.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", p.Tool, principal.ToolDataEntry)
	}
	if got := p.PrincipalType(); got != principal.TypePeer {
		t.Errorf("PrincipalType = %q, want %q", got, principal.TypePeer)
	}
	if !principal.IsReserved(p.User) {
		t.Error("a peer identity must be reserved")
	}
	if principal.IsRelaIdentity(p.User) {
		t.Error("a peer is not one of rela's own identities")
	}
}

// TestPrincipalType_DerivedFromPeerName pins that the name decides the type,
// and that the claim alone keeps it after the principal_property lookup
// replaces the user name with an entity id.
func TestPrincipalType_DerivedFromPeerName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		p    principal.Principal
		want string
	}{
		{"bare peer name", principal.Principal{User: "peer:acme"}, principal.TypePeer},
		{"claim cannot override the name",
			principal.VerifiedFrom("peer:acme", principal.ToolDataEntry,
				principal.Claims{PrincipalType: "user"}),
			principal.TypePeer},
		{"empty peer name is not a peer", principal.Principal{User: "peer:"}, ""},
		{"resolved entity id keeps the claim",
			principal.VerifiedFrom("USER-1", principal.ToolDataEntry,
				principal.Claims{PrincipalType: principal.TypePeer}),
			principal.TypePeer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.p.PrincipalType(); got != tc.want {
				t.Errorf("PrincipalType = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsReserved_Peer(t *testing.T) {
	t.Parallel()
	for user, want := range map[string]bool{
		"peer:acme":     true,
		"peer:":         true,
		"\x01peer:x":    true,
		"Peer:acme":     false,
		"my-peer:acme":  false,
		"peeracme":      false,
		"integration:x": true,
	} {
		if got := principal.IsReserved(user); got != want {
			t.Errorf("IsReserved(%q) = %v, want %v", user, got, want)
		}
	}
}

func TestReservedPrefixes_IsACopy(t *testing.T) {
	t.Parallel()
	got := principal.ReservedPrefixes()
	want := []string{principal.ReservedPrefix, principal.IntegrationPrefix, principal.PeerPrefix}
	if !slices.Equal(got, want) {
		t.Fatalf("ReservedPrefixes = %v, want %v", got, want)
	}
	got[0] = "x"
	if !principal.IsReserved("system:scheduler") {
		t.Error("mutating the returned slice changed IsReserved")
	}
}

func TestValidateRunAs_RefusesPeer(t *testing.T) {
	t.Parallel()
	if err := principal.ValidateRunAs("peer:acme", func(string) bool { return true }); err == nil {
		t.Error("run_as peer:acme was accepted")
	}
	if err := principal.ValidJobIdentity("peer:acme"); err == nil {
		t.Error("peer:acme was accepted as a job identity")
	}
}

func TestValidPeerName(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"acme": true, "acme-eu_1": true, "": false, "Acme": false, "-acme": false, "a.b": false, "con": false,
	} {
		if got := principal.ValidPeerName(name); got != want {
			t.Errorf("ValidPeerName(%q) = %v, want %v", name, got, want)
		}
	}
}
