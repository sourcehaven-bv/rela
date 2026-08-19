package metamodel

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestACLBypass_Unmarshal pins the accepted spellings and the capability
// each one grants.
func TestACLBypass_Unmarshal(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		yaml      string
		want      ACLBypass
		wantRead  bool
		wantWrite bool
	}{
		{name: "read", yaml: "read", want: ACLBypassRead, wantRead: true},
		{name: "write", yaml: "write", want: ACLBypassWrite, wantWrite: true},
		{
			name: "read+write", yaml: "read+write", want: ACLBypassReadWrite,
			wantRead: true, wantWrite: true,
		},
		{name: "empty string is none", yaml: `""`, want: ACLBypassNone},
		// Case and surrounding space are forgiven: the value is
		// operator-typed, and a capitalised READ is unambiguous.
		{name: "uppercase", yaml: "READ", want: ACLBypassRead, wantRead: true},
		{name: "padded", yaml: `"  read+write  "`, want: ACLBypassReadWrite, wantRead: true, wantWrite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got ACLBypass
			if err := yaml.Unmarshal([]byte(tc.yaml), &got); err != nil {
				t.Fatalf("Unmarshal(%s): %v", tc.yaml, err)
			}
			if got != tc.want {
				t.Errorf("value = %q, want %q", got, tc.want)
			}
			if got.AllowsRead() != tc.wantRead {
				t.Errorf("AllowsRead() = %v, want %v", got.AllowsRead(), tc.wantRead)
			}
			if got.AllowsWrite() != tc.wantWrite {
				t.Errorf("AllowsWrite() = %v, want %v", got.AllowsWrite(), tc.wantWrite)
			}
			if got.Enabled() != (tc.wantRead || tc.wantWrite) {
				t.Errorf("Enabled() = %v, want %v", got.Enabled(), tc.wantRead || tc.wantWrite)
			}
		})
	}
}

// TestACLBypass_RejectsLegacyBool is the load-bearing one. `true` used to
// mean read+write, and silently reinterpreting it would resolve a privilege
// field toward MORE access if the two spellings ever drifted. The refusal
// must name the replacement so the operator is not left guessing.
func TestACLBypass_RejectsLegacyBool(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"true", "false"} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			var got ACLBypass
			err := yaml.Unmarshal([]byte(in), &got)
			if err == nil {
				t.Fatalf("Unmarshal(%s) succeeded (value %q); a boolean must be refused, "+
					"never silently mapped to a capability", in, got)
			}
			// The message has to be actionable: it names the new spelling and
			// the command that rewrites existing files.
			for _, want := range []string{"read+write", "rela migrate"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

// TestACLBypass_RejectsUnknown pins that an unrecognized value fails loudly
// rather than degrading to no-elevation. A typo'd capability that silently
// meant "none" would be a script that quietly stops working; a typo that
// silently meant "read+write" would be far worse.
func TestACLBypass_RejectsUnknown(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"all", "readwrite", "read,write", "rw", "create"} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			var got ACLBypass
			if err := yaml.Unmarshal([]byte(in), &got); err == nil {
				t.Errorf("Unmarshal(%q) succeeded (value %q); want an error", in, got)
			}
		})
	}
}

// TestACLBypass_ZeroValueGrantsNothing pins the default. A field an operator
// never wrote must not elevate.
func TestACLBypass_ZeroValueGrantsNothing(t *testing.T) {
	t.Parallel()

	var zero ACLBypass
	if zero.Enabled() || zero.AllowsRead() || zero.AllowsWrite() {
		t.Errorf("zero value grants something: enabled=%v read=%v write=%v",
			zero.Enabled(), zero.AllowsRead(), zero.AllowsWrite())
	}
}

// TestACLBypass_AbsentKeyLeavesNone pins that omitting the key entirely (the
// overwhelmingly common case) leaves the action unelevated — the
// UnmarshalYAML hook must not run and invent a value.
func TestACLBypass_AbsentKeyLeavesNone(t *testing.T) {
	t.Parallel()

	var action AutomationAction
	if err := yaml.Unmarshal([]byte("lua: |\n  print('x')\n"), &action); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if action.AllowACLBypass.Enabled() {
		t.Errorf("an action with no allow_acl_bypass key is elevated (%q)", action.AllowACLBypass)
	}
}
