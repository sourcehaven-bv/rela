package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanCleanup_SafeRemoval(t *testing.T) {
	// Type with no instances and no references - safe to remove
	analysis := &Analysis{
		UnusedEntityTypes: []TypeUsage{
			{Name: "orphan-type", Count: 0, References: nil},
		},
		UnusedRelationTypes: []TypeUsage{
			{Name: "orphan-rel", Count: 0, References: nil},
		},
		UnusedCustomTypes: []TypeUsage{
			{Name: "orphan-enum", Count: 0, References: nil},
		},
	}

	plan := PlanCleanup(analysis)

	if len(plan.MetamodelChanges) != 3 {
		t.Fatalf("expected 3 metamodel changes, got %d", len(plan.MetamodelChanges))
	}

	// Check entity type removal
	found := false
	for _, change := range plan.MetamodelChanges {
		if change.Action == "remove_entity_type" && change.Target == "orphan-type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected remove_entity_type for orphan-type")
	}

	// Check relation type removal
	found = false
	for _, change := range plan.MetamodelChanges {
		if change.Action == "remove_relation_type" && change.Target == "orphan-rel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected remove_relation_type for orphan-rel")
	}

	// Check custom type removal
	found = false
	for _, change := range plan.MetamodelChanges {
		if change.Action == "remove_custom_type" && change.Target == "orphan-enum" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected remove_custom_type for orphan-enum")
	}
}

func TestPlanCleanup_CascadesReferencedTypes(t *testing.T) {
	// Type with no instances but referenced in form - should be removed with cascade
	analysis := &Analysis{
		UnusedEntityTypes: []TypeUsage{
			{
				Name:  "referenced-type",
				Count: 0,
				References: []Reference{
					{File: "data-entry.yaml", Section: "forms.my-form", Kind: "form"},
				},
			},
		},
	}

	plan := PlanCleanup(analysis)

	// Entity type should be removed from metamodel
	var foundEntityRemoval bool
	for _, change := range plan.MetamodelChanges {
		if change.Action == "remove_entity_type" && change.Target == "referenced-type" {
			foundEntityRemoval = true
			break
		}
	}
	if !foundEntityRemoval {
		t.Error("expected entity type to be removed")
	}

	// Form should be cascade-removed from data-entry.yaml
	var foundFormRemoval bool
	for _, change := range plan.DataEntryChanges {
		if change.Action == "remove_form" && change.Target == "my-form" {
			foundFormRemoval = true
			break
		}
	}
	if !foundFormRemoval {
		t.Error("expected form to be cascade-removed")
	}
}

func TestPlanCleanup_CascadeMultipleReferences(t *testing.T) {
	// Type with multiple references - all should be cascade-removed
	analysis := &Analysis{
		UnusedEntityTypes: []TypeUsage{
			{
				Name:  "multi-ref-type",
				Count: 0,
				References: []Reference{
					{File: "data-entry.yaml", Section: "forms.type-form", Kind: "form"},
					{File: "data-entry.yaml", Section: "lists.type-list", Kind: "list"},
					{File: "metamodel.yaml", Section: "relations.some-rel.from", Kind: "relation_from"},
				},
			},
		},
	}

	plan := PlanCleanup(analysis)

	// Entity type should be removed
	var foundEntityRemoval bool
	for _, change := range plan.MetamodelChanges {
		if change.Action == "remove_entity_type" && change.Target == "multi-ref-type" {
			foundEntityRemoval = true
			break
		}
	}
	if !foundEntityRemoval {
		t.Error("expected entity type to be removed")
	}

	// Form and list should be cascade-removed from data-entry.yaml
	var foundForm, foundList bool
	for _, change := range plan.DataEntryChanges {
		if change.Action == "remove_form" && change.Target == "type-form" {
			foundForm = true
		}
		if change.Action == "remove_list" && change.Target == "type-list" {
			foundList = true
		}
	}
	if !foundForm {
		t.Error("expected form to be cascade-removed")
	}
	if !foundList {
		t.Error("expected list to be cascade-removed")
	}
}

func TestPlanCleanup_PreservesTypesWithInstances(t *testing.T) {
	// Type with instances but in unused list (shouldn't happen but test safety)
	analysis := &Analysis{
		UnusedEntityTypes: []TypeUsage{
			{Name: "has-instances", Count: 5, References: nil},
		},
	}

	plan := PlanCleanup(analysis)

	for _, change := range plan.MetamodelChanges {
		if change.Target == "has-instances" {
			t.Error("should not plan removal of type with instances")
		}
	}
}

func TestCleanupPlan_TotalChanges(t *testing.T) {
	plan := &CleanupPlan{
		MetamodelChanges: []Change{{}, {}},
		DataEntryChanges: []Change{{}},
	}

	if plan.TotalChanges() != 3 {
		t.Errorf("expected TotalChanges=3, got %d", plan.TotalChanges())
	}
}

func TestCleanupPlan_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		plan *CleanupPlan
		want bool
	}{
		{
			name: "empty plan",
			plan: &CleanupPlan{},
			want: true,
		},
		{
			name: "has metamodel changes",
			plan: &CleanupPlan{MetamodelChanges: []Change{{}}},
			want: false,
		},
		{
			name: "has data entry changes",
			plan: &CleanupPlan{DataEntryChanges: []Change{{}}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecuteCleanup_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test metamodel.yaml
	metamodelContent := `entities:
  requirement:
    properties:
      title:
        type: string
  to-remove:
    properties:
      title:
        type: string
relations:
  implements:
    from: [requirement]
    to: [requirement]
  to-remove-rel:
    from: [requirement]
    to: [requirement]
types:
  status:
    values: [draft, done]
  to-remove-enum:
    values: [a, b, c]
`
	metamodelPath := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(metamodelPath, []byte(metamodelContent), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := &CleanupPlan{
		MetamodelChanges: []Change{
			{File: "metamodel.yaml", Action: "remove_entity_type", Target: "to-remove"},
			{File: "metamodel.yaml", Action: "remove_relation_type", Target: "to-remove-rel"},
			{File: "metamodel.yaml", Action: "remove_custom_type", Target: "to-remove-enum"},
		},
	}

	// Execute with dry run
	if err := ExecuteCleanup(plan, tmpDir, true); err != nil {
		t.Fatalf("ExecuteCleanup dry run failed: %v", err)
	}

	// File should not have changed
	content, err := os.ReadFile(metamodelPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "to-remove:") {
		t.Error("dry run should not modify file")
	}
}

func TestExecuteCleanup_AppliesChanges(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test metamodel.yaml
	metamodelContent := `entities:
  requirement:
    properties:
      title:
        type: string
  to-remove:
    properties:
      title:
        type: string
relations:
  implements:
    from: [requirement]
    to: [requirement]
  to-remove-rel:
    from: [requirement]
    to: [requirement]
types:
  status:
    values: [draft, done]
  to-remove-enum:
    values: [a, b, c]
`
	metamodelPath := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(metamodelPath, []byte(metamodelContent), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := &CleanupPlan{
		MetamodelChanges: []Change{
			{File: "metamodel.yaml", Action: "remove_entity_type", Target: "to-remove"},
			{File: "metamodel.yaml", Action: "remove_relation_type", Target: "to-remove-rel"},
			{File: "metamodel.yaml", Action: "remove_custom_type", Target: "to-remove-enum"},
		},
	}

	// Execute for real
	if err := ExecuteCleanup(plan, tmpDir, false); err != nil {
		t.Fatalf("ExecuteCleanup failed: %v", err)
	}

	// Check file was modified
	content, err := os.ReadFile(metamodelPath)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)

	if strings.Contains(contentStr, "to-remove:") {
		t.Error("entity type 'to-remove' should have been removed")
	}
	if strings.Contains(contentStr, "to-remove-rel:") {
		t.Error("relation type 'to-remove-rel' should have been removed")
	}
	if strings.Contains(contentStr, "to-remove-enum:") {
		t.Error("custom type 'to-remove-enum' should have been removed")
	}

	// Preserved types should still exist
	if !strings.Contains(contentStr, "requirement:") {
		t.Error("entity type 'requirement' should be preserved")
	}
	if !strings.Contains(contentStr, "implements:") {
		t.Error("relation type 'implements' should be preserved")
	}
	if !strings.Contains(contentStr, "status:") {
		t.Error("custom type 'status' should be preserved")
	}
}

func TestExecuteCleanup_EmptyPlan(t *testing.T) {
	// Empty plan should return early without error
	plan := &CleanupPlan{}
	if err := ExecuteCleanup(plan, "/nonexistent", false); err != nil {
		t.Errorf("empty plan should not error: %v", err)
	}
}

func TestExecuteCleanup_CascadeDataEntry(t *testing.T) {
	tmpDir := t.TempDir()

	// Create metamodel.yaml
	metamodelContent := `entities:
  requirement:
    properties:
      title:
        type: string
  to-remove:
    properties:
      title:
        type: string
`
	metamodelPath := filepath.Join(tmpDir, "metamodel.yaml")
	if err := os.WriteFile(metamodelPath, []byte(metamodelContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create data-entry.yaml with form and list for to-remove type
	dataEntryContent := `forms:
  requirement-form:
    entity_type: requirement
  to-remove-form:
    entity_type: to-remove
lists:
  requirement-list:
    entity_type: requirement
  to-remove-list:
    entity_type: to-remove
`
	dataEntryPath := filepath.Join(tmpDir, "data-entry.yaml")
	if err := os.WriteFile(dataEntryPath, []byte(dataEntryContent), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := &CleanupPlan{
		MetamodelChanges: []Change{
			{File: "metamodel.yaml", Action: "remove_entity_type", Target: "to-remove"},
		},
		DataEntryChanges: []Change{
			{File: "data-entry.yaml", Action: "remove_form", Target: "to-remove-form"},
			{File: "data-entry.yaml", Action: "remove_list", Target: "to-remove-list"},
		},
	}

	if err := ExecuteCleanup(plan, tmpDir, false); err != nil {
		t.Fatalf("ExecuteCleanup failed: %v", err)
	}

	// Check metamodel.yaml
	metamodelData, err := os.ReadFile(metamodelPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metamodelData), "to-remove:") {
		t.Error("entity type 'to-remove' should have been removed from metamodel")
	}

	// Check data-entry.yaml
	dataEntryData, err := os.ReadFile(dataEntryPath)
	if err != nil {
		t.Fatal(err)
	}
	dataEntryStr := string(dataEntryData)
	if strings.Contains(dataEntryStr, "to-remove-form:") {
		t.Error("form 'to-remove-form' should have been removed")
	}
	if strings.Contains(dataEntryStr, "to-remove-list:") {
		t.Error("list 'to-remove-list' should have been removed")
	}
	if !strings.Contains(dataEntryStr, "requirement-form:") {
		t.Error("form 'requirement-form' should be preserved")
	}
	if !strings.Contains(dataEntryStr, "requirement-list:") {
		t.Error("list 'requirement-list' should be preserved")
	}
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		section string
		prefix  string
		want    string
	}{
		{"forms.my-form", "forms.", "my-form"},
		{"lists.my-list", "lists.", "my-list"},
		{"forms.my-form.relations", "forms.", "my-form"},
		{"validations.my-val", "validations.", "my-val"},
		{"unknown", "forms.", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.section, func(t *testing.T) {
			if got := extractName(tt.section, tt.prefix); got != tt.want {
				t.Errorf("extractName(%q, %q) = %q, want %q", tt.section, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestCanSafelyRemove(t *testing.T) {
	tests := []struct {
		name  string
		usage TypeUsage
		want  bool
	}{
		{
			name:  "no instances no references",
			usage: TypeUsage{Count: 0, References: nil},
			want:  true,
		},
		{
			name:  "has instances",
			usage: TypeUsage{Count: 1, References: nil},
			want:  false,
		},
		{
			name: "no instances with references - still removable",
			usage: TypeUsage{
				Count: 0,
				References: []Reference{
					{Kind: "form"},
					{Kind: "list"},
					{Kind: "view"},
				},
			},
			want: true, // References are cascade-removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canSafelyRemove(tt.usage); got != tt.want {
				t.Errorf("canSafelyRemove() = %v, want %v", got, tt.want)
			}
		})
	}
}
