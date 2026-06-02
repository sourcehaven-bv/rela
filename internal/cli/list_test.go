package cli

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// list_test.go only covers CLI-specific behavior: entity-type resolution
// via aliases/plurals. Pure graph iteration (ListByType / AllNodes /
// empty-store) is covered by the store conformance suite in
// internal/store/storetest/query.go and does not need to be duplicated
// here.

func TestResolveEntityTypeWithAlias(t *testing.T) {
	meta, err := metamodel.Parse([]byte(testutil.AliasMetamodelYAML()))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}

	tests := []struct {
		name      string
		input     string
		wantType  string
		wantError bool
	}{
		{name: "canonical name", input: "requirement", wantType: "requirement"},
		{name: "alias req", input: "req", wantType: "requirement"},
		{name: "plural form", input: "requirements", wantType: "requirement"},
		{name: "alias ctrl", input: "ctrl", wantType: "control"},
		{name: "plural controls", input: "controls", wantType: "control"},
		{name: "unknown type", input: "unknown", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, _, err := resolveEntityType(meta, tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("resolveEntityType(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("resolveEntityType(%q) unexpected error: %v", tt.input, err)
				return
			}
			if resolved != tt.wantType {
				t.Errorf("resolveEntityType(%q) = %q, want %q", tt.input, resolved, tt.wantType)
			}
		})
	}
}

// TestListTypeParsingEdgeCases tests edge cases for entity type resolution
// including entity types and aliases that end in 's' (like "bus", "autobus").
func TestListTypeParsingEdgeCases(t *testing.T) {
	meta := testutil.NewMetamodel().
		DefineEntity("requirement").
		Label("Requirement").
		IDPrefix("REQ-").
		Aliases("req").
		Prop("title", metamodel.PropertyTypeString, true).
		Prop("status", "status", true).
		End().
		DefineEntity("bus").
		Label("Bus").
		IDPrefix("BUS-").
		Aliases("autobus").
		Prop("title", metamodel.PropertyTypeString, true).
		End().
		WithCustomTypeDefault("status", []string{"draft", "accepted"}, "draft").
		Build()

	tests := []struct {
		name      string
		input     string
		wantType  string
		wantError bool
	}{
		{name: "canonical name requirement", input: "requirement", wantType: "requirement"},
		{name: "alias req", input: "req", wantType: "requirement"},
		{name: "plural requirements", input: "requirements", wantType: "requirement"},
		{name: "canonical name bus (ends in s)", input: "bus", wantType: "bus"},
		{name: "alias autobus (ends in s)", input: "autobus", wantType: "bus"},
		{name: "plural buses", input: "buses", wantType: "bus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, _, err := resolveEntityType(meta, tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("resolveEntityType(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("resolveEntityType(%q) unexpected error: %v", tt.input, err)
				return
			}
			if resolved != tt.wantType {
				t.Errorf("resolveEntityType(%q) = %q, want %q", tt.input, resolved, tt.wantType)
			}
		})
	}
}

func TestListCommandWithUnknownType(t *testing.T) {
	meta := metamodel.DefaultMetamodel()

	_, _, err := resolveEntityType(meta, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown entity type")
	}
	if !strings.Contains(err.Error(), "unknown entity type") {
		t.Errorf("expected 'unknown entity type' in error, got: %v", err)
	}
}
