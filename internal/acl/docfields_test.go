package acl_test

import (
	"bytes"
	"log/slog"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// docFieldsPolicyYAML exercises both prose doc-fields added in rela-docs
// phase 1b (TKT-JO2SAD): a top-level `description` and a per-role
// `description`. The `viewer` role deliberately has no description so the
// tests can confirm an absent field stays empty rather than defaulting.
const docFieldsPolicyYAML = `
description: >
  Access model for the demo tracker. Editors maintain everything; viewers
  read only.
roles:
  editor:
    description: >
      Full maintainer. May create, update, and delete every entity type.
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
  viewer:
    read: ["*"]
assignments:
  alice: editor
  bob: viewer
`

// AC1/AC2: both descriptions parse into their struct fields, and a role
// without a description is empty (not defaulted).
func TestLoadPolicy_DocFields_Present(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicy(writeTempPolicy(t, docFieldsPolicyYAML))
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	// AC2: top-level policy description.
	if p.Description == "" {
		t.Error("Policy.Description should be populated from `description:`")
	}
	// AC1: per-role description.
	if p.Roles["editor"].Description == "" {
		t.Error("editor role Description should be populated")
	}
	// A role with no `description:` entry is simply empty — not an error.
	if got := p.Roles["viewer"].Description; got != "" {
		t.Errorf("viewer role Description = %q, want empty", got)
	}
	// The descriptions must not disturb the grant fields they sit beside.
	if got := p.Roles["editor"].Create; len(got) != 1 || got[0] != "*" {
		t.Errorf("editor.Create = %v, want [*]", got)
	}
}

// AC5: the in-tree example policy (the corpus the phase-2 `rela docs` generator
// will run against) loads clean and carries the prose fields.
func TestLoadPolicy_PrototypeExample(t *testing.T) {
	t.Parallel()
	p, err := acl.LoadPolicy("../../prototypes/data-entry/project/acl.yaml")
	if err != nil {
		t.Fatalf("LoadPolicy(prototype acl.yaml): %v", err)
	}
	if p.Description == "" {
		t.Error("prototype policy should carry a top-level description")
	}
	if p.Roles["editor"].Description == "" || p.Roles["viewer"].Description == "" {
		t.Error("prototype roles editor+viewer should each carry a description")
	}
}

// AC4 (absence): a policy with neither field loads with empty descriptions and
// a nil Validate() — byte-for-byte the pre-feature behavior.
func TestLoadPolicy_DocFields_Absent(t *testing.T) {
	t.Parallel()
	const yaml = `
roles:
  viewer:
    read: ["*"]
assignments:
  bob: viewer
`
	p, err := acl.LoadPolicy(writeTempPolicy(t, yaml))
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.Description != "" {
		t.Errorf("Policy.Description = %q, want empty", p.Description)
	}
	if p.Roles["viewer"].Description != "" {
		t.Errorf("viewer.Description = %q, want empty", p.Roles["viewer"].Description)
	}
	// Validate() is unaffected by the prose fields.
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

// AC4 (round-trip): parse -> marshal -> parse preserves both descriptions.
func TestPolicyDocFields_RoundTrip(t *testing.T) {
	t.Parallel()
	p1, err := acl.LoadPolicy(writeTempPolicy(t, docFieldsPolicyYAML))
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	out, err := yaml.Marshal(p1)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	p2, err := acl.LoadPolicyBytes(out)
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	if p2.Description != p1.Description {
		t.Errorf("round-trip Policy.Description = %q, want %q", p2.Description, p1.Description)
	}
	if p2.Roles["editor"].Description != p1.Roles["editor"].Description {
		t.Errorf("round-trip editor.Description = %q, want %q",
			p2.Roles["editor"].Description, p1.Roles["editor"].Description)
	}
}

// AC3: the top-level `description` key is a KNOWN key — LoadPolicy must NOT emit
// the "unknown key" warning for it (regression guard for the knownPolicyKeys
// allowlist entry). A genuinely unknown key sitting beside it still warns, so
// the test also proves the warning path is live (not globally suppressed).
//
// NOT parallel: swaps the process-global default slog logger (same class as
// t.Setenv), mirroring TestLoadPolicy_UnknownKey_LogsWarning.
func TestLoadPolicy_DescriptionKeyNotWarned(t *testing.T) {
	const yaml = `
description: A documented deployment.
roles:
  viewer:
    read: ["*"]
bogus_key: oops
`
	path := writeTempPolicy(t, yaml)

	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	p, err := acl.LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.Description != "A documented deployment." {
		t.Errorf("Policy.Description = %q, want it populated", p.Description)
	}

	logs := buf.String()
	if contains(logs, "description") {
		t.Errorf("`description` must not be warned as unknown; logs:\n%s", logs)
	}
	// Sanity-negative: a real unknown key still warns, so we know the warning
	// path is exercised and not silently disabled.
	if !contains(logs, "bogus_key") {
		t.Errorf("expected the genuinely-unknown `bogus_key` to warn; logs:\n%s", logs)
	}
}
