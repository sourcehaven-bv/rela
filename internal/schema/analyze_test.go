package schema

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// mockCounter implements TypeCounter for tests.
type mockCounter struct {
	entities  map[string]int
	relations map[string]int
}

func (m *mockCounter) CountByEntityType(t string) int   { return m.entities[t] }
func (m *mockCounter) CountByRelationType(t string) int { return m.relations[t] }

func newTestMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string", Required: true},
					"status":   {Type: "req-status"},
					"priority": {Type: "priority"},
				},
			},
			"decision": {
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "status"},
				},
			},
			"unused-type": {
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				From: []string{"decision"},
				To:   []string{"requirement"},
			},
			"depends-on": {
				From: []string{"requirement"},
				To:   []string{"requirement"},
			},
			"unused-relation": {
				From: []string{"requirement"},
				To:   []string{"decision"},
			},
		},
		Types: map[string]metamodel.CustomType{
			"req-status": {
				Values: []string{"draft", "approved", "implemented"},
			},
			"priority": {
				Values: []string{"low", "medium", "high"},
			},
			"unused-enum": {
				Values: []string{"a", "b", "c"},
			},
		},
	}
}

func newTestCounter() *mockCounter {
	return &mockCounter{
		entities: map[string]int{
			"requirement": 2,
			"decision":    1,
		},
		relations: map[string]int{
			"implements": 1,
			"depends-on": 1,
		},
	}
}

func TestAnalyze_UnusedEntityTypes(t *testing.T) {
	meta := newTestMetamodel()
	c := newTestCounter()

	result := Analyze(meta, c, nil, 0)

	// unused-type has no instances
	if len(result.UnusedEntityTypes) != 1 {
		t.Fatalf("expected 1 unused entity type, got %d", len(result.UnusedEntityTypes))
	}
	if result.UnusedEntityTypes[0].Name != "unused-type" {
		t.Errorf("expected unused-type, got %s", result.UnusedEntityTypes[0].Name)
	}
	if result.UnusedEntityTypes[0].Count != 0 {
		t.Errorf("expected count 0, got %d", result.UnusedEntityTypes[0].Count)
	}
}

func TestAnalyze_UnusedRelationTypes(t *testing.T) {
	meta := newTestMetamodel()
	c := newTestCounter()

	result := Analyze(meta, c, nil, 0)

	// unused-relation has no instances
	if len(result.UnusedRelationTypes) != 1 {
		t.Fatalf("expected 1 unused relation type, got %d", len(result.UnusedRelationTypes))
	}
	if result.UnusedRelationTypes[0].Name != "unused-relation" {
		t.Errorf("expected unused-relation, got %s", result.UnusedRelationTypes[0].Name)
	}
}

func TestAnalyze_UnusedCustomTypes(t *testing.T) {
	meta := newTestMetamodel()
	c := newTestCounter()

	result := Analyze(meta, c, nil, 0)

	// unused-enum is not referenced by any property
	if len(result.UnusedCustomTypes) != 1 {
		t.Fatalf("expected 1 unused custom type, got %d", len(result.UnusedCustomTypes))
	}
	if result.UnusedCustomTypes[0].Name != "unused-enum" {
		t.Errorf("expected unused-enum, got %s", result.UnusedCustomTypes[0].Name)
	}
}

func TestAnalyze_LowUsageThreshold(t *testing.T) {
	meta := newTestMetamodel()
	c := newTestCounter()

	// With threshold=1, decision (1 instance) should be in low usage
	result := Analyze(meta, c, nil, 1)

	var found bool
	for _, usage := range result.LowUsageEntityTypes {
		if usage.Name == "decision" {
			found = true
			if usage.Count != 1 {
				t.Errorf("expected decision count 1, got %d", usage.Count)
			}
		}
	}
	if !found {
		t.Error("expected decision to be in low usage types")
	}

	// requirement has 2 instances, should not be in low usage at threshold=1
	for _, usage := range result.LowUsageEntityTypes {
		if usage.Name == "requirement" {
			t.Error("requirement should not be in low usage at threshold=1")
		}
	}
}

func TestAnalyze_WithDataEntryConfig(t *testing.T) {
	meta := newTestMetamodel()
	c := &mockCounter{} // Empty counts - all types unused

	dataEntry := &dataentryconfig.Config{
		Forms: map[string]dataentryconfig.Form{
			"req-form": {
				EntityType: "requirement",
			},
		},
		Lists: map[string]dataentryconfig.List{
			"req-list": {
				EntityType: "requirement",
			},
		},
	}

	result := Analyze(meta, c, dataEntry, 0)

	// requirement should have references in data-entry.yaml
	var reqUsage *TypeUsage
	for i := range result.UnusedEntityTypes {
		if result.UnusedEntityTypes[i].Name == "requirement" {
			reqUsage = &result.UnusedEntityTypes[i]
			break
		}
	}

	if reqUsage == nil {
		t.Fatal("expected requirement to be in unused entity types")
	}

	// Check for form and list references
	var hasFormRef, hasListRef bool
	for _, ref := range reqUsage.References {
		if ref.Kind == "form" {
			hasFormRef = true
		}
		if ref.Kind == "list" {
			hasListRef = true
		}
	}

	if !hasFormRef {
		t.Error("expected form reference")
	}
	if !hasListRef {
		t.Error("expected list reference")
	}
}

func TestAnalyze_MetamodelValidationReferences(t *testing.T) {
	meta := newTestMetamodel()
	meta.Validations = []metamodel.ValidationRule{
		{
			Name:       "req-needs-title",
			EntityType: "requirement",
		},
	}
	c := &mockCounter{}

	result := Analyze(meta, c, nil, 0)

	// requirement should have validation reference
	var reqUsage *TypeUsage
	for i := range result.UnusedEntityTypes {
		if result.UnusedEntityTypes[i].Name == "requirement" {
			reqUsage = &result.UnusedEntityTypes[i]
			break
		}
	}

	if reqUsage == nil {
		t.Fatal("expected requirement in unused types")
	}

	var hasValidationRef bool
	for _, ref := range reqUsage.References {
		if ref.Kind == "validation" {
			hasValidationRef = true
			break
		}
	}

	if !hasValidationRef {
		t.Error("expected validation reference")
	}
}

func TestAnalyze_MetamodelAutomationReferences(t *testing.T) {
	meta := newTestMetamodel()
	meta.Automations = []metamodel.AutomationDef{
		{
			Name: "auto-impl",
			On: metamodel.AutomationTrigger{
				Entity: []string{"requirement"},
			},
		},
	}
	c := &mockCounter{}

	result := Analyze(meta, c, nil, 0)

	// requirement should have automation reference
	var reqUsage *TypeUsage
	for i := range result.UnusedEntityTypes {
		if result.UnusedEntityTypes[i].Name == "requirement" {
			reqUsage = &result.UnusedEntityTypes[i]
			break
		}
	}

	if reqUsage == nil {
		t.Fatal("expected requirement in unused types")
	}

	var hasAutomationRef bool
	for _, ref := range reqUsage.References {
		if ref.Kind == "automation" {
			hasAutomationRef = true
			break
		}
	}

	if !hasAutomationRef {
		t.Error("expected automation reference")
	}
}

func TestAnalysis_TotalUnused(t *testing.T) {
	analysis := &Analysis{
		UnusedEntityTypes:   []TypeUsage{{Name: "a"}, {Name: "b"}},
		UnusedRelationTypes: []TypeUsage{{Name: "c"}},
		UnusedCustomTypes:   []TypeUsage{{Name: "d"}, {Name: "e"}, {Name: "f"}},
	}

	if analysis.TotalUnused() != 6 {
		t.Errorf("expected TotalUnused=6, got %d", analysis.TotalUnused())
	}
}

func TestAnalysis_TotalLowUsage(t *testing.T) {
	analysis := &Analysis{
		LowUsageEntityTypes:   []TypeUsage{{Name: "a"}},
		LowUsageRelationTypes: []TypeUsage{{Name: "b"}, {Name: "c"}},
	}

	if analysis.TotalLowUsage() != 3 {
		t.Errorf("expected TotalLowUsage=3, got %d", analysis.TotalLowUsage())
	}
}

func TestAnalysis_HasIssues(t *testing.T) {
	tests := []struct {
		name     string
		analysis *Analysis
		want     bool
	}{
		{
			name:     "no issues",
			analysis: &Analysis{},
			want:     false,
		},
		{
			name: "has unused entity types",
			analysis: &Analysis{
				UnusedEntityTypes: []TypeUsage{{Name: "a"}},
			},
			want: true,
		},
		{
			name: "has low usage types",
			analysis: &Analysis{
				LowUsageEntityTypes: []TypeUsage{{Name: "a"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.analysis.HasIssues(); got != tt.want {
				t.Errorf("HasIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyze_SortsResults(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"zebra":  {},
			"apple":  {},
			"mango":  {},
			"banana": {},
		},
		Relations: map[string]metamodel.RelationDef{},
		Types:     map[string]metamodel.CustomType{},
	}
	c := &mockCounter{}

	result := Analyze(meta, c, nil, 0)

	// Check alphabetical order
	expected := []string{"apple", "banana", "mango", "zebra"}
	for i, usage := range result.UnusedEntityTypes {
		if usage.Name != expected[i] {
			t.Errorf("index %d: expected %s, got %s", i, expected[i], usage.Name)
		}
	}
}
