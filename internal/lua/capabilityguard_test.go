package lua

import (
	"os"
	"path/filepath"
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

	// Read EVERY non-test source in the package, not just runtime.go: the
	// functions being guarded live in ai.go and http.go, so a registration
	// added from a new mode file (or from ai.go itself) would be invisible to
	// a single-file grep.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var sb strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, readErr := os.ReadFile(filepath.Clean(name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		sb.Write(b)
		sb.WriteString("\n")
	}
	text := sb.String()

	tests := []struct {
		name   string
		needle string // the registration call
		gate   string // the guard that must appear before it
	}{
		{
			name:   "ai module",
			needle: "r.registerAIModule()",
			gate:   "if r.caps.AI {",
		},
		{
			name:   "http module",
			needle: "r.registerHTTPModule()",
			gate:   "if r.caps.HTTP {",
		},
		{
			name:   "write_file binding",
			needle: `SetField(rela, "write_file"`,
			gate:   "if r.caps.WriteFile {",
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
