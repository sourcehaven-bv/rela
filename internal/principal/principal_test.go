package principal_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func TestPrincipalFrom_DefaultsToUnknown(t *testing.T) {
	got := principal.From(context.Background())
	want := principal.Principal{User: "unknown", Tool: "unknown"}
	if !got.Equal(want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestWithPrincipal_RoundTrip(t *testing.T) {
	p := principal.Principal{User: "alice", Tool: principal.ToolCLI}
	ctx := principal.With(context.Background(), p)
	if got := principal.From(ctx); !got.Equal(p) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, p)
	}
}

// Stamped distinguishes "no identity on this ctx" from "an identity that
// happens to be unknown" — a distinction From deliberately erases. The MCP
// server's principal middleware depends on it to decide whether to supply a
// fallback identity, so an unstamped ctx and a literally-unknown one must not
// look the same.
func TestStamped_DistinguishesAbsentFromUnknown(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		got, ok := principal.Stamped(context.Background())
		if ok {
			t.Errorf("ok = true for an unstamped ctx, want false (got %+v)", got)
		}
	})

	t.Run("an explicitly unknown principal IS stamped", func(t *testing.T) {
		// The case that makes From insufficient: this is indistinguishable
		// from absent under From, but a caller was genuinely stamped here.
		p := principal.Principal{User: "unknown", Tool: "unknown"}
		got, ok := principal.Stamped(principal.With(context.Background(), p))

		if !ok {
			t.Fatal("ok = false, want true — the principal was stamped, even " +
				"though its value is unknown/unknown")
		}
		if !got.Equal(p) {
			t.Errorf("got %+v, want %+v", got, p)
		}
	})

	t.Run("round-trips a real principal", func(t *testing.T) {
		p := principal.Principal{User: "alice", Tool: principal.ToolMCP}
		got, ok := principal.Stamped(principal.With(context.Background(), p))
		if !ok || !got.Equal(p) {
			t.Errorf("got (%+v, %v), want (%+v, true)", got, ok, p)
		}
	})
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
