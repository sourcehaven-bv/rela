package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCapabilities_FromRealYAML is the operator's-eye view of TKT-YH52OM: it
// starts from data-entry.yaml text, not from a Go struct literal, so it proves
// the documented spelling actually decodes.
//
// The three docs that tell operators to write this block (the Lua guide, the
// IdP webhook guide, and the config godoc) are only correct if this passes.
func TestCapabilities_FromRealYAML(t *testing.T) {
	t.Parallel()

	const src = `
actions:
  notify_slack:
    label: "Notify Slack"
    script: notify.lua
    capabilities:
      http: true
      secrets: [slack_webhook_url]
  inert:
    label: "No capabilities"
    script: inert.lua
documents:
  sales_report:
    title: "Sales report"
    script: docs/sales.lua
    capabilities:
      ai: true
      secrets: [openai_key]
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("decode data-entry.yaml: %v", err)
	}

	slack := cfg.Actions["notify_slack"]
	if !slack.Capabilities.HTTP {
		t.Error("http: true did not decode onto the action")
	}
	if got := slack.Capabilities.Secrets; len(got) != 1 || got[0] != "slack_webhook_url" {
		t.Errorf("secrets list: got %v, want [slack_webhook_url]", got)
	}
	// The point of a named list: this action must NOT be able to reach a
	// secret it did not name.
	if slack.Capabilities.AI || slack.Capabilities.WriteFile {
		t.Error("undeclared capabilities must stay off")
	}

	// An action with no block reaches nothing.
	if cfg.Actions["inert"].Capabilities.Any() {
		t.Errorf("an action with no capabilities: block must grant nothing, got %+v",
			cfg.Actions["inert"].Capabilities)
	}

	// Documents use the identical spelling.
	doc := cfg.Documents["sales_report"]
	if !doc.Capabilities.AI {
		t.Error("ai: true did not decode onto the document")
	}
	if doc.Capabilities.HTTP {
		t.Error("document must not receive http it never declared")
	}
}

// TestCapabilities_BareBoolRefusedInContext pins that the parse-time refusal
// survives being nested inside a real config, not just unmarshalled alone —
// an operator reaching for `capabilities: true` gets a message naming the
// mapping form rather than a silent grant-everything.
func TestCapabilities_BareBoolRefusedInContext(t *testing.T) {
	t.Parallel()

	const src = `
actions:
  sloppy:
    script: x.lua
    capabilities: true
`
	var cfg Config
	err := yaml.Unmarshal([]byte(src), &cfg)
	if err == nil {
		t.Fatal("`capabilities: true` must be refused, not silently accepted")
	}
	if !strings.Contains(err.Error(), "must be a mapping") {
		t.Errorf("error should name the mapping form, got: %v", err)
	}
	// The message must point at the fix, since this is the error an operator
	// hits when migrating.
	if !strings.Contains(err.Error(), "secrets") {
		t.Errorf("error should show the secrets spelling, got: %v", err)
	}
}
