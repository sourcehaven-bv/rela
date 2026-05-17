package audit_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

func TestSystemUser(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{"unset", "", "unknown"},
		{"normal", "alice", "alice"},
		{"trims whitespace", "  bob  ", "bob"},
		{"whitespace-only is unknown", "   ", "unknown"},
		{"newline-only is unknown", "\n", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("USER", tt.env)
			if got := audit.SystemUser(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
