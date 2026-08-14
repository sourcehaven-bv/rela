package cli

import (
	"strings"
	"testing"
)

// TestACLMapCmd_ClientViewValidation pins the flag combinations that must FAIL
// rather than silently print the wrong thing.
//
// The failure being prevented is specific: a flag named --as that promises
// attenuation but produces an un-attenuated map would tell an operator a
// restricted client has access it does not, or vice versa. An attestation tool
// that quietly ignores a flag is worse than one that refuses.
func TestACLMapCmd_ClientViewValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		cmd       ACLMapCmd
		wantErr   string
		wantType  string
		wantScope []string
	}{
		{
			name: "no attenuation flags",
			cmd:  ACLMapCmd{Principal: "alice"},
		},
		{
			name:     "as with principal",
			cmd:      ACLMapCmd{Principal: "alice", As: "app"},
			wantType: "app",
		},
		{
			name:      "space-delimited scopes, as the claim spells them",
			cmd:       ACLMapCmd{Principal: "alice", As: "app", Scope: "rela.read rela.tickets.write"},
			wantType:  "app",
			wantScope: []string{"rela.read", "rela.tickets.write"},
		},
		{
			name:      "comma-delimited scopes, as a shell user types them",
			cmd:       ACLMapCmd{Principal: "alice", As: "app", Scope: "rela.read,rela.tickets.write"},
			wantType:  "app",
			wantScope: []string{"rela.read", "rela.tickets.write"},
		},
		{
			name:    "as without principal is refused",
			cmd:     ACLMapCmd{As: "app"},
			wantErr: "--principal",
		},
		{
			name:    "scope without as is refused",
			cmd:     ACLMapCmd{Principal: "alice", Scope: "rela.read"},
			wantErr: "--scope requires --as",
		},
		{
			name:    "scope without principal is refused",
			cmd:     ACLMapCmd{Scope: "rela.read"},
			wantErr: "--principal",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			view, err := tc.cmd.clientView()
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("clientView() = %+v, want an error containing %q", view, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("clientView() error = %v", err)
			}
			if view.PrincipalType != tc.wantType {
				t.Errorf("PrincipalType = %q, want %q", view.PrincipalType, tc.wantType)
			}
			if len(view.Scopes) != len(tc.wantScope) {
				t.Fatalf("Scopes = %v, want %v", view.Scopes, tc.wantScope)
			}
			for i := range tc.wantScope {
				if view.Scopes[i] != tc.wantScope[i] {
					t.Errorf("Scopes[%d] = %q, want %q", i, view.Scopes[i], tc.wantScope[i])
				}
			}
		})
	}
}

// TestSplitScopes covers the whitespace shapes a claim or a shell can produce.
func TestSplitScopes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"   ", 0},
		{"a", 1},
		{"a b", 2},
		{"a  b", 2},  // repeated separators collapse
		{" a b ", 2}, // leading/trailing ignored
		{"a,b", 2},   // comma form
		{"a, b", 2},  // mixed
		{"a\tb\nc", 3},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := splitScopes(tc.in); len(got) != tc.want {
				t.Errorf("splitScopes(%q) = %v (%d), want %d elements", tc.in, got, len(got), tc.want)
			}
		})
	}
}
