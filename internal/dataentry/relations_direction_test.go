package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// resolveDirection wraps Metamodel.InverseOwner with a canonical-first
// precedence and a symmetric-relation override. Tests cover the four
// shapes a body key can have:
//
//   - matches a canonical relation name
//   - matches an inverse name of a non-symmetric relation
//   - matches an inverse name of a symmetric relation (self-inverse)
//   - matches neither — caller surfaces a structural error
func TestResolveDirection(t *testing.T) {
	// Build a metamodel by hand to avoid YAML noise. The two-level
	// init (inverseOwners isn't exported) is intentional — we test
	// the runtime helper, not the loader.
	meta := buildDirectionTestMetamodel(t)

	t.Run("canonical name resolves outgoing", func(t *testing.T) {
		canonical, incoming, ok := resolveDirection(meta, "blocks")
		if !ok {
			t.Fatal("expected ok=true for canonical name")
		}
		if canonical != "blocks" {
			t.Errorf("canonical = %q, want %q", canonical, "blocks")
		}
		if incoming {
			t.Errorf("canonical name should resolve outgoing, got incoming=true")
		}
	})

	t.Run("inverse name resolves incoming", func(t *testing.T) {
		canonical, incoming, ok := resolveDirection(meta, "blockedBy")
		if !ok {
			t.Fatal("expected ok=true for known inverse")
		}
		if canonical != "blocks" {
			t.Errorf("canonical = %q, want %q", canonical, "blocks")
		}
		if !incoming {
			t.Errorf("inverse name should resolve incoming, got incoming=false")
		}
	})

	t.Run("symmetric self-inverse resolves outgoing", func(t *testing.T) {
		canonical, incoming, ok := resolveDirection(meta, "related-to")
		if !ok {
			t.Fatal("expected ok=true for symmetric relation")
		}
		if canonical != "related-to" {
			t.Errorf("canonical = %q, want %q", canonical, "related-to")
		}
		// Both the canonical-first precedence AND the symmetric
		// override force outgoing here; this test pins the
		// invariant either way.
		if incoming {
			t.Errorf("symmetric should resolve outgoing, got incoming=true")
		}
	})

	t.Run("unknown name returns ok=false", func(t *testing.T) {
		canonical, incoming, ok := resolveDirection(meta, "does-not-exist")
		if ok {
			t.Errorf("expected ok=false for unknown name, got canonical=%q incoming=%v", canonical, incoming)
		}
	})
}

// buildDirectionTestMetamodel constructs a metamodel with three
// relations exercising every direction branch: one with a distinct
// inverse name, one symmetric self-inverse, one with no inverse.
// Mirrors the load-time validation so the inverseOwners map is
// populated by the same path the runtime uses.
func buildDirectionTestMetamodel(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	yaml := `version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "D-"
    properties:
      title:
        type: string
        required: true
relations:
  blocks:
    label: blocks
    from: [doc]
    to: [doc]
    inverse: blockedBy
  related-to:
    label: related to
    from: [doc]
    to: [doc]
    symmetric: true
    inverse: related-to
  affects:
    label: affects
    from: [doc]
    to: [doc]
`
	m, err := metamodel.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse test metamodel: %v", err)
	}
	return m
}
