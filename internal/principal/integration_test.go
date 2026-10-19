package principal_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TKT-01KZSO AC6: the integration namespace is reserved like system:.
func TestIsReserved_Integration(t *testing.T) {
	tests := []struct {
		user string
		want bool
	}{
		{"integration:basecamp", true},
		{"integration:", true},
		{" \x01integration:basecamp", true},
		{"Integration:basecamp", false},
		{"my-integration:basecamp", false},
		{"integrations:basecamp", false},
	}
	for _, tt := range tests {
		if got := principal.IsReserved(tt.user); got != tt.want {
			t.Errorf("IsReserved(%q) = %v, want %v", tt.user, got, tt.want)
		}
	}
}

func TestValidIntegrationName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"basecamp", true},
		{"b", true},
		{"0crm", true},
		{"crm_2-eu", true},
		{strings.Repeat("a", 63), true},
		{strings.Repeat("a", 64), false},
		{"", false},
		{"-crm", false},
		{"_crm", false},
		{"Basecamp", false},
		{"base camp", false},
		{"base/camp", false},
		{"base.camp", false},
		{"base:camp", false},
		{"bäsecamp", false},
		{"con", false},
		{"nul", false},
		{"com1", false},
		{"lpt9", false},
		{"console", true},
	}
	for _, tt := range tests {
		if got := principal.ValidIntegrationName(tt.name); got != tt.want {
			t.Errorf("ValidIntegrationName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestValidateRunAs(t *testing.T) {
	onlyAutomation := func(s string) bool { return s == principal.UserAutomation }
	tests := []struct {
		name    string
		runAs   string
		allow   func(string) bool
		wantErr string
	}{
		{"plain user", "bot", nil, ""},
		{"integration", "integration:basecamp", nil, ""},
		{"integration bad grammar", "integration:Basecamp", nil, "integration name"},
		{"integration empty", "integration:", nil, "integration name"},
		{"integration reserved device", "integration:con", nil, "integration name"},
		{"system refused by default", "system:scheduler", nil, "rela's own"},
		{"system allowed by caller", "system:automation", onlyAutomation, ""},
		{"other system refused", "system:scheduler", onlyAutomation, "rela's own"},
		{"leading space", " bot", nil, "space"},
		{"empty", "", nil, "empty"},
		{"control char", "b\x01ot", nil, "control"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := principal.ValidateRunAs(tt.runAs, tt.allow)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateRunAs(%q) = %v, want nil", tt.runAs, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateRunAs(%q) = %v, want error containing %q", tt.runAs, err, tt.wantErr)
			}
		})
	}
}

func TestRefuseRunAsEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{name: "unset", env: map[string]string{}},
		{name: "empty", env: map[string]string{principal.RunAsEnv: ""}},
		{name: "set", env: map[string]string{principal.RunAsEnv: "system:ci"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := principal.RefuseRunAsEnv(func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("RefuseRunAsEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), principal.RunAsEnv) {
				t.Errorf("error %q does not name %s", err, principal.RunAsEnv)
			}
		})
	}
}

func TestIsRelaIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		user string
		want bool
	}{
		{principal.UserRealtime, true}, {principal.UserScheduler, true}, {principal.UserAutomation, true}, {principal.UserProvisioner, true},
		{principal.UserMail, true}, {principal.UserFSImport, true}, {principal.UserGCSweep, true}, {principal.UserPerfSeed, true},
		{"system:webhook:github", true}, {"system:subscription:news", true},
		{"system:ci", false}, {"system:job:nightly", false}, {"integration:bot", false}, {"alice", false},
	} {
		if got := principal.IsRelaIdentity(tc.user); got != tc.want {
			t.Errorf("principal.IsRelaIdentity(%q) = %v, want %v", tc.user, got, tc.want)
		}
	}
}

func TestValidSystemName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"ci", true}, {"job:nightly", true}, {"a-b_c1", true},
		{"", false}, {"job:", false}, {":ci", false}, {"Ci", false}, {"c i", false},
		{"realtime\u200b", false}, {"\u202erealtime", false}, {"r\u0435altime", false},
	} {
		if got := principal.ValidSystemName(tc.name); got != tc.want {
			t.Errorf("principal.ValidSystemName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
