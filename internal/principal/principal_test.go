package principal_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func TestPrincipalFrom_DefaultsToUnknown(t *testing.T) {
	got := principal.From(context.Background())
	want := principal.Principal{User: "unknown", Tool: "unknown"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestWithPrincipal_RoundTrip(t *testing.T) {
	p := principal.Principal{User: "alice", Tool: principal.ToolCLI}
	ctx := principal.With(context.Background(), p)
	if got := principal.From(ctx); got != p {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, p)
	}
}

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
			if got := principal.SystemUser(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
