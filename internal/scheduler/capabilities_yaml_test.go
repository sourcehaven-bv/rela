package scheduler

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestTaskCapabilities_FromRealYAML proves the `capabilities:` block decodes
// from schedules.yaml text (TKT-YH52OM). A scheduled job is the surface most
// likely to want outbound HTTP, and it is unattended, so the grant has to be
// declarable here and must default to nothing.
func TestTaskCapabilities_FromRealYAML(t *testing.T) {
	t.Parallel()

	const src = `
tasks:
  - name: nightly-sync
    script: sync.lua
    every: day
    capabilities:
      http: true
      secrets: [upstream_token]
  - name: local-tidy
    script: tidy.lua
    every: day
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("decode schedules.yaml: %v", err)
	}
	if len(cfg.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(cfg.Tasks))
	}

	sync := cfg.Tasks[0]
	if !sync.Capabilities.HTTP {
		t.Error("http: true did not decode onto the task")
	}
	if got := sync.Capabilities.Secrets; len(got) != 1 || got[0] != "upstream_token" {
		t.Errorf("secrets: got %v, want [upstream_token]", got)
	}

	// A task that declares nothing reaches nothing — RunAs (identity) must not
	// be confused for a capability grant.
	if cfg.Tasks[1].Capabilities.Any() {
		t.Errorf("task with no capabilities: block must grant nothing, got %+v",
			cfg.Tasks[1].Capabilities)
	}
}
