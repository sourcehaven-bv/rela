package migration

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGetMapValue(t *testing.T) {
	yamlContent := `
key1: value1
key2: value2
nested:
  inner: innervalue
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	root := GetDocumentRoot(&doc)

	tests := []struct {
		name    string
		key     string
		wantVal string
		wantNil bool
	}{
		{name: "existing key", key: "key1", wantVal: "value1"},
		{name: "another existing key", key: "key2", wantVal: "value2"},
		{name: "non-existing key", key: "key3", wantNil: true},
		{name: "nested key", key: "nested", wantNil: false}, // Returns the nested map
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetMapValue(root, tt.key)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetMapValue() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Error("GetMapValue() = nil, want non-nil")
				} else if tt.wantVal != "" && got.Value != tt.wantVal {
					t.Errorf("GetMapValue().Value = %q, want %q", got.Value, tt.wantVal)
				}
			}
		})
	}
}

func TestGetMapValue_NilAndNonMap(t *testing.T) {
	// Test nil node
	if got := GetMapValue(nil, "key"); got != nil {
		t.Error("GetMapValue(nil) should return nil")
	}

	// Test non-map node
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(`- item1`), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	root := GetDocumentRoot(&doc)
	if got := GetMapValue(root, "key"); got != nil {
		t.Error("GetMapValue on sequence should return nil")
	}
}

func TestSetMapValue(t *testing.T) {
	yamlContent := `
key1: value1
key2: value2
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	root := GetDocumentRoot(&doc)

	// Update existing key
	SetMapValue(root, "key1", "newvalue1")
	if got := GetMapValue(root, "key1"); got == nil || got.Value != "newvalue1" {
		t.Errorf("SetMapValue didn't update existing key")
	}

	// Add new key
	SetMapValue(root, "key3", "value3")
	if got := GetMapValue(root, "key3"); got == nil || got.Value != "value3" {
		t.Errorf("SetMapValue didn't add new key")
	}
}

func TestRenameMapKey(t *testing.T) {
	yamlContent := `
oldkey: value
other: othervalue
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	root := GetDocumentRoot(&doc)

	// Rename existing key
	if !RenameMapKey(root, "oldkey", "newkey") {
		t.Error("RenameMapKey should return true for existing key")
	}

	if got := GetMapValue(root, "oldkey"); got != nil {
		t.Error("old key should not exist after rename")
	}

	if got := GetMapValue(root, "newkey"); got == nil || got.Value != "value" {
		t.Error("new key should exist with original value")
	}

	// Try to rename non-existing key
	if RenameMapKey(root, "nonexistent", "something") {
		t.Error("RenameMapKey should return false for non-existing key")
	}
}

func TestFindMapEntriesByKey(t *testing.T) {
	yamlContent := `
entities:
  req:
    id_type: sequential
  comp:
    id_type: string
  other:
    label: Other
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	entries := FindMapEntriesByKey(&doc, "id_type")

	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}

	values := make(map[string]bool)
	for _, entry := range entries {
		values[entry[1].Value] = true
	}

	if !values["sequential"] || !values["string"] {
		t.Error("expected to find both 'sequential' and 'string' values")
	}
}

func TestReplaceMapValueByKey(t *testing.T) {
	yamlContent := `
entities:
  req:
    id_type: sequential
  comp:
    id_type: string
  req2:
    id_type: sequential
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Replace all 'sequential' with 'auto'
	count := ReplaceMapValueByKey(&doc, "id_type", "sequential", "auto")

	if count != 2 {
		t.Errorf("replaced %d values, want 2", count)
	}

	// Verify replacements
	entries := FindMapEntriesByKey(&doc, "id_type")
	for _, entry := range entries {
		if entry[1].Value == "sequential" {
			t.Error("found unreplaced 'sequential' value")
		}
	}

	// Count 'auto' values
	autoCount := 0
	for _, entry := range entries {
		if entry[1].Value == "auto" {
			autoCount++
		}
	}
	if autoCount != 2 {
		t.Errorf("got %d 'auto' values, want 2", autoCount)
	}
}

func TestWalkMappings(t *testing.T) {
	yamlContent := `
level1:
  level2:
    level3:
      key: value
  another: value
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	count := 0
	WalkMappings(&doc, func(_ *yaml.Node) bool {
		count++
		return true
	})

	// Should visit: root, level1, level2, level3
	if count != 4 {
		t.Errorf("visited %d mappings, want 4", count)
	}

	// Test early termination
	count = 0
	WalkMappings(&doc, func(_ *yaml.Node) bool {
		count++
		return count < 2 // Stop after 2
	})

	if count != 2 {
		t.Errorf("early termination: visited %d mappings, want 2", count)
	}
}

func TestFindScalarValues(t *testing.T) {
	yamlContent := `
entities:
  req:
    id_type: sequential
  comp:
    id_type: sequential
  other:
    id_type: string
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	found := FindScalarValues(&doc, "sequential")
	if len(found) != 2 {
		t.Errorf("found %d occurrences of 'sequential', want 2", len(found))
	}

	found = FindScalarValues(&doc, "string")
	if len(found) != 1 {
		t.Errorf("found %d occurrences of 'string', want 1", len(found))
	}

	found = FindScalarValues(&doc, "nonexistent")
	if len(found) != 0 {
		t.Errorf("found %d occurrences of 'nonexistent', want 0", len(found))
	}
}

func TestGetDocumentRoot(t *testing.T) {
	yamlContent := `key: value`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	root := GetDocumentRoot(&doc)
	if root == nil {
		t.Fatal("GetDocumentRoot returned nil")
	}

	if root.Kind != yaml.MappingNode {
		t.Errorf("root kind = %d, want %d (MappingNode)", root.Kind, yaml.MappingNode)
	}

	// Test nil
	if GetDocumentRoot(nil) != nil {
		t.Error("GetDocumentRoot(nil) should return nil")
	}
}
