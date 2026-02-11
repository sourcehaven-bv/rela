package filter

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// TestMatchAll_MalformedEntityData verifies that filtering handles malformed entity data gracefully.
// This test documents the expected behavior: entities with data that cannot be evaluated
// against the filter should return an error that callers can handle (e.g., skip the entity).
func TestMatchAll_MalformedEntityData(t *testing.T) {
	mm := &metamodel.Metamodel{}
	entityDef := &metamodel.EntityDef{
		Properties: map[string]metamodel.PropertyDef{
			"implementation_cost": {Type: metamodel.PropertyTypeInteger},
			"title":               {Type: metamodel.PropertyTypeString, Required: true},
			"status":              {Type: metamodel.PropertyTypeEnum, Values: []string{"draft", "accepted"}},
		},
	}

	tests := []struct {
		name        string
		entity      *model.Entity
		filterExpr  string
		wantMatch   bool
		wantErr     bool
		errContains string
	}{
		{
			name: "valid entity matches filter",
			entity: &model.Entity{
				ID:   "CTRL-001",
				Type: "control",
				Properties: map[string]interface{}{
					"implementation_cost": 15000,
					"title":               "Valid Control",
					"status":              "accepted",
				},
			},
			filterExpr: "implementation_cost>10000",
			wantMatch:  true,
			wantErr:    false,
		},
		{
			name: "valid entity does not match filter",
			entity: &model.Entity{
				ID:   "CTRL-002",
				Type: "control",
				Properties: map[string]interface{}{
					"implementation_cost": 5000,
					"title":               "Cheap Control",
					"status":              "accepted",
				},
			},
			filterExpr: "implementation_cost>10000",
			wantMatch:  false,
			wantErr:    false,
		},
		{
			name: "malformed integer - string where int expected",
			entity: &model.Entity{
				ID:   "CTRL-BAD",
				Type: "control",
				Properties: map[string]interface{}{
					"implementation_cost": "not-a-number",
					"title":               "Bad Entity",
					"status":              "draft",
				},
			},
			filterExpr:  "implementation_cost>10000",
			wantMatch:   false,
			wantErr:     true,
			errContains: "invalid integer",
		},
		{
			name: "malformed integer - array where int expected",
			entity: &model.Entity{
				ID:   "CTRL-BAD2",
				Type: "control",
				Properties: map[string]interface{}{
					"implementation_cost": []string{"a", "b"},
					"title":               "Bad Entity 2",
					"status":              "draft",
				},
			},
			filterExpr:  "implementation_cost>10000",
			wantMatch:   false,
			wantErr:     true,
			errContains: "cannot parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, err := ParseAll([]string{tt.filterExpr})
			if err != nil {
				t.Fatalf("ParseAll(%q) error: %v", tt.filterExpr, err)
			}

			got, err := MatchAll(tt.entity, filters, entityDef, mm)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MatchAll() expected error containing %q, got nil", tt.errContains)
					return
				}
				// Error is expected - this is the current behavior
				// The caller should handle this gracefully (skip entity, not crash)
				return
			}

			if err != nil {
				t.Fatalf("MatchAll() unexpected error: %v", err)
			}

			if got != tt.wantMatch {
				t.Errorf("MatchAll() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

// TestMatchAll_MixedValidAndMalformedEntities demonstrates the scenario where
// a list of entities contains some with valid data and some with malformed data.
// The filter loop should skip malformed entities rather than failing entirely.
func TestMatchAll_MixedValidAndMalformedEntities(t *testing.T) {
	mm := &metamodel.Metamodel{}
	entityDef := &metamodel.EntityDef{
		Properties: map[string]metamodel.PropertyDef{
			"implementation_cost": {Type: metamodel.PropertyTypeInteger},
			"title":               {Type: metamodel.PropertyTypeString, Required: true},
			"status":              {Type: metamodel.PropertyTypeEnum, Values: []string{"draft", "accepted"}},
		},
	}

	entities := []*model.Entity{
		{
			ID:   "CTRL-001",
			Type: "control",
			Properties: map[string]interface{}{
				"implementation_cost": 15000,
				"title":               "Valid Control 1",
				"status":              "accepted",
			},
		},
		{
			ID:   "CTRL-BAD",
			Type: "control",
			Properties: map[string]interface{}{
				"implementation_cost": "not-a-number", // Malformed!
				"title":               "Bad Entity",
				"status":              "draft",
			},
		},
		{
			ID:   "CTRL-002",
			Type: "control",
			Properties: map[string]interface{}{
				"implementation_cost": 20000,
				"title":               "Valid Control 2",
				"status":              "accepted",
			},
		},
	}

	filters, err := ParseAll([]string{"implementation_cost>10000"})
	if err != nil {
		t.Fatalf("ParseAll error: %v", err)
	}

	// Simulate what the list command should do: filter entities, skipping malformed ones
	var filtered []*model.Entity
	var skipped []string

	for _, e := range entities {
		matches, err := MatchAll(e, filters, entityDef, mm)
		if err != nil {
			// Entity has malformed data - skip it (this is the expected graceful handling)
			skipped = append(skipped, e.ID)
			continue
		}
		if matches {
			filtered = append(filtered, e)
		}
	}

	// We should get the two valid matching entities
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered entities, got %d", len(filtered))
	}

	// We should have skipped the malformed entity
	if len(skipped) != 1 || skipped[0] != "CTRL-BAD" {
		t.Errorf("Expected to skip CTRL-BAD, skipped: %v", skipped)
	}

	// Verify the correct entities were returned
	expectedIDs := map[string]bool{"CTRL-001": true, "CTRL-002": true}
	for _, e := range filtered {
		if !expectedIDs[e.ID] {
			t.Errorf("Unexpected entity in results: %s", e.ID)
		}
	}
}
