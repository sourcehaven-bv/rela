package automation

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestCapabilitiesSurviveActionConversion is a REGRESSION test (TKT-YH52OM).
//
// convertFromMetamodel turns a metamodel.AutomationAction into the internal Action, and the
// conversion is a hand-written field list. It originally copied AllowACLBypass
// but NOT Capabilities, so every automation's `capabilities:` block was
// silently dropped one hop before it could reach the runtime — the script then
// failed with "attempt to index a nil value (global 'http')" despite the
// operator having declared it.
//
// This is the second instance of the same class of bug in this ticket (the
// first being the scheduler's deps grant), which is why the translation now
// goes through metamodel.Capabilities.Fields.
func TestCapabilitiesSurviveActionConversion(t *testing.T) {
	t.Parallel()

	def := metamodel.AutomationDef{
		Name: "notify",
		Do: []metamodel.AutomationAction{{
			Lua: "return 1",
			Capabilities: metamodel.Capabilities{
				HTTP:    true,
				Secrets: []string{"slack"},
			},
		}},
	}

	got := convertFromMetamodel(def)
	if len(got.Do) != 1 {
		t.Fatalf("expected 1 action, got %d", len(got.Do))
	}
	caps := got.Do[0].Capabilities
	if !caps.HTTP {
		t.Error("http grant was dropped converting metamodel.Action -> Action")
	}
	if len(caps.Secrets) != 1 || caps.Secrets[0] != "slack" {
		t.Errorf("secrets grant was dropped: got %v, want [slack]", caps.Secrets)
	}
}
