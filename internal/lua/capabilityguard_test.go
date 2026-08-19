package lua

import (
	"os"
	"strings"
	"testing"
)

// TestCapabilityRegistrationStaysGated is a source-level guard, in the spirit
// of dataentry's TestNoStrayWriteRequestConstruction: it fails if a capability
// binding is ever registered outside its `if r.caps...` gate (TKT-YH52OM).
//
// Why a grep guard rather than only behavioral tests: the behavioral tests
// assert what a runtime built through NewReader/NewWriter exposes. A future
// change that adds a SECOND registration path — a new mode, a convenience
// constructor, a "just for tests" helper — can reintroduce the ungated binding
// without failing any of them, because they never construct that new path.
// The four registrations below are the only ones that may exist, and each must
// sit under its gate.
//
// If this fails, do not delete it. Either put the new registration behind the
// capability check, or (if there is genuinely a new legitimate site) extend the
// expected counts here with a comment saying why.
func TestCapabilityRegistrationStaysGated(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("runtime.go")
	if err != nil {
		t.Fatalf("read runtime.go: %v", err)
	}
	text := string(src)

	tests := []struct {
		name     string
		needle   string   // the registration call
		gate     string   // the guard that must appear before it
		wantOnce bool     // registration must appear exactly once
		absent   []string // spellings that must NOT reappear
	}{
		{
			name:     "ai module",
			needle:   "r.registerAIModule()",
			gate:     "if r.caps.AI {",
			wantOnce: true,
		},
		{
			name:     "http module",
			needle:   "r.registerHTTPModule()",
			gate:     "if r.caps.HTTP {",
			wantOnce: true,
		},
		{
			name:     "write_file binding",
			needle:   `SetField(rela, "write_file"`,
			gate:     "if r.caps.WriteFile {",
			wantOnce: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if n := strings.Count(text, tc.needle); n != 1 {
				t.Fatalf("%s: expected exactly 1 registration of %q, found %d — "+
					"a second registration site can bypass the capability gate",
					tc.name, tc.needle, n)
			}
			gateIdx := strings.Index(text, tc.gate)
			if gateIdx < 0 {
				t.Fatalf("%s: capability gate %q is gone — the binding is "+
					"unconditional again", tc.name, tc.gate)
			}
			regIdx := strings.Index(text, tc.needle)
			if regIdx < gateIdx {
				t.Errorf("%s: registration appears before its gate %q",
					tc.name, tc.gate)
			}
			// The gate must be close to the registration; a distant match would
			// mean the guard is checking an unrelated block.
			if regIdx-gateIdx > 200 {
				t.Errorf("%s: gate %q is %d bytes before the registration — "+
					"is the registration still inside it?", tc.name, tc.gate, regIdx-gateIdx)
			}
		})
	}

	// Secrets are filtered, never ranged over raw. `range r.secrets` is exactly
	// the mutation that reintroduces whole-file exposure.
	t.Run("secrets are filtered", func(t *testing.T) {
		t.Parallel()
		if !strings.Contains(text, "r.caps.filterSecrets(r.secrets)") {
			t.Error("the secrets table is no longer built through filterSecrets — " +
				"every script may now read the whole of .rela/secrets.yaml")
		}
		if strings.Contains(text, "range r.secrets {") {
			t.Error("found `range r.secrets` — the secrets table must be built " +
				"from caps.filterSecrets, not from the raw map")
		}
	})
}
