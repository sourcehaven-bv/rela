package principal_test

import (
	"context"
	"strings"
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

func TestIsReserved(t *testing.T) {
	tests := []struct {
		name string
		user string
		want bool
	}{
		// The two identities that exist today. Both are grantable in acl.yaml,
		// which is precisely why neither may arrive from a request path.
		{"scheduler constant", principal.UserScheduler, true},
		{"provisioner constant", principal.UserProvisioner, true},

		// The whole namespace is reserved, not just the known constants, so a
		// future system:* identity is safe before anyone updates a list.
		{"unknown system identity", "system:future", true},
		{"bare prefix", "system:", true},

		// Leading noise must not defeat the check. This is the standalone-
		// correctness property: a caller may pass a RAW value (provision.go
		// does), and a caller's own sanitizer may map noise onto a reserved
		// name — data-entry's sanitizeUser turns "\x01system:scheduler" into
		// exactly "system:scheduler". If IsReserved only trimmed whitespace,
		// whether a name counted as reserved would depend on whether the caller
		// sanitized first: safe by ordering, not by construction (RR-NQK412).
		{"leading space", "  system:scheduler", true},
		{"trailing space", "system:scheduler  ", true},
		{"tab indented", "\tsystem:scheduler", true},
		{"leading C0 control char", "\x01system:scheduler", true},
		{"leading DEL", "\x7fsystem:scheduler", true},
		{"leading NUL", "\x00system:scheduler", true},
		{"mixed leading noise", " \x01\t system:scheduler", true},

		// Only the LEADING run is stripped. Interior noise cannot manufacture a
		// prefix, and stripping it would conflate names the ACL treats as
		// distinct (its assignment lookup is an exact map index).
		{"interior control char", "sys\x01tem:scheduler", false},
		{"control char after prefix", "system:\x01scheduler", true},

		// Case-sensitive by design: acl.yaml assignments match exactly, so
		// this confers no scheduler grant and is an ordinary username.
		// Rejecting it would cost a real user their login for no security gain.
		{"capitalized is not reserved", "System:Scheduler", false},
		{"uppercase is not reserved", "SYSTEM:SCHEDULER", false},

		// Near-misses that must keep working.
		{"no colon", "systemscheduler", false},
		{"prefix not at start", "my-system:scheduler", false},
		{"different namespace", "webhook:user-created", false},
		{"ordinary user", "alice", false},
		{"email shaped", "alice@example.com", false},
		{"empty", "", false},
		{"whitespace only", "   ", false},

		// A fullwidth colon is not the ASCII prefix. It also matches no
		// assignments key, so treating it as ordinary escalates nothing.
		{"unicode lookalike colon", "system：scheduler", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := principal.IsReserved(tt.user); got != tt.want {
				t.Errorf("principal.IsReserved(%q) = %v, want %v", tt.user, got, tt.want)
			}
		})
	}
}

// TestIsReserved_CoversEveryReservedConstant fails if a new system:* constant
// is added that IsReserved does not classify as reserved. The prefix rule makes
// that structurally impossible today; this pins the property so a future change
// to either the constants or the predicate cannot silently break it.
func TestIsReserved_CoversEveryReservedConstant(t *testing.T) {
	for _, user := range []string{principal.UserScheduler, principal.UserProvisioner} {
		if !strings.HasPrefix(user, principal.ReservedPrefix) {
			t.Errorf("constant %q does not carry principal.ReservedPrefix %q", user, principal.ReservedPrefix)
		}
		if !principal.IsReserved(user) {
			t.Errorf("principal.IsReserved(%q) = false, want true", user)
		}
	}
}
