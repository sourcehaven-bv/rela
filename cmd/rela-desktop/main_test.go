package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"hello-world", "Hello World"},
		{"hello_world", "Hello World"},
		{"my-test-case", "My Test Case"},
		{"snake_case_name", "Snake Case Name"},
		{"mixed-case_name", "Mixed Case Name"},
		{"", ""},
		{"single", "Single"},
		{"UPPERCASE", "UPPERCASE"},
		{"already Title Case", "Already Title Case"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := titleCase(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestScanForRelaProjects(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create project directories with metamodel.yaml
	project1 := filepath.Join(tmpDir, "project1")
	project2 := filepath.Join(tmpDir, "nested", "project2")
	require.NoError(t, os.MkdirAll(project1, 0o755))
	require.NoError(t, os.MkdirAll(project2, 0o755))

	// Create metamodel.yaml files
	require.NoError(t, os.WriteFile(filepath.Join(project1, "metamodel.yaml"), []byte("entities: {}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(project2, "metamodel.yaml"), []byte("entities: {}"), 0o644))

	// Create a hidden directory that should be skipped
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	require.NoError(t, os.MkdirAll(hiddenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "metamodel.yaml"), []byte("entities: {}"), 0o644))

	// Create node_modules that should be skipped
	nodeModules := filepath.Join(tmpDir, "node_modules", "some-package")
	require.NoError(t, os.MkdirAll(nodeModules, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nodeModules, "metamodel.yaml"), []byte("entities: {}"), 0o644))

	// Scan for projects
	projects := scanForRelaProjects(tmpDir)

	// Should find exactly 2 projects (not hidden or node_modules)
	assert.Len(t, projects, 2)
	assert.Contains(t, projects, project1)
	assert.Contains(t, projects, project2)
}

func TestScanForRelaProjects_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	projects := scanForRelaProjects(tmpDir)
	assert.Empty(t, projects)
}

func TestScanForRelaProjects_NonExistentDir(t *testing.T) {
	projects := scanForRelaProjects("/nonexistent/path")
	assert.Empty(t, projects)
}

func TestGenerateDataEntryConfig(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"feature": {
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string"},
					"status":   {Type: "string"},
					"priority": {Type: "string"},
				},
			},
			"bug": {
				Properties: map[string]metamodel.PropertyDef{
					"title":       {Type: "string"},
					"description": {Type: "text"},
				},
			},
		},
	}

	config := generateDataEntryConfig("Test App", meta)

	// Check app name (new format uses app.name)
	assert.Contains(t, config, "name: \"Test App\"")

	// Check forms section
	assert.Contains(t, config, "forms:")
	assert.Contains(t, config, "  feature:")
	assert.Contains(t, config, "    entity_type: feature")
	assert.Contains(t, config, "    title: \"Feature\"")
	assert.Contains(t, config, "  bug:")
	assert.Contains(t, config, "    entity_type: bug")
	assert.Contains(t, config, "    title: \"Bug\"")

	// Check lists section
	assert.Contains(t, config, "lists:")
	assert.Contains(t, config, "  features:")
	assert.Contains(t, config, "    entity_type: feature")
	assert.Contains(t, config, "  bugs:")
	assert.Contains(t, config, "    entity_type: bug")

	// Check navigation section
	assert.Contains(t, config, "navigation:")
	assert.Contains(t, config, "  - label: \"Features\"")
	assert.Contains(t, config, "    list: features")
	assert.Contains(t, config, "  - label: \"Bugs\"")
	assert.Contains(t, config, "    list: bugs")

	// Check properties exist (they're sorted alphabetically)
	assert.Contains(t, config, "property: priority")
	assert.Contains(t, config, "property: status")
	assert.Contains(t, config, "property: title")
}

func TestGenerateDataEntryConfig_EmptyMetamodel(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{},
	}

	config := generateDataEntryConfig("Empty App", meta)

	assert.Contains(t, config, "name: \"Empty App\"")
	assert.Contains(t, config, "forms:")
	assert.Contains(t, config, "lists:")
	assert.Contains(t, config, "navigation:")
}

func TestGenerateDataEntryConfig_KebabCaseEntityType(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"test-case": {
				Properties: map[string]metamodel.PropertyDef{
					"test-name": {Type: "string"},
				},
			},
		},
	}

	config := generateDataEntryConfig("Test App", meta)

	// Kebab-case should be converted to underscore for form/list IDs
	assert.Contains(t, config, "  test_case:")
	assert.Contains(t, config, "    entity_type: test-case")
	assert.Contains(t, config, "    title: \"Test Case\"")
	assert.Contains(t, config, "  test_cases:")
	assert.Contains(t, config, "  - label: \"Test Cases\"")
	assert.Contains(t, config, "    list: test_cases")
}

func TestGenerateDataEntryConfig_MaxColumns(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"entity": {
				Properties: map[string]metamodel.PropertyDef{
					"a": {Type: "string"},
					"b": {Type: "string"},
					"c": {Type: "string"},
					"d": {Type: "string"},
					"e": {Type: "string"},
					"f": {Type: "string"},
				},
			},
		},
	}

	config := generateDataEntryConfig("Test App", meta)

	// Lists should have max 4 columns
	// Find the entitys list section between "  entitys:" and "create_form:"
	listsIdx := strings.Index(config, "lists:")
	require.NotEqual(t, -1, listsIdx, "should have lists section")

	entitysIdx := strings.Index(config[listsIdx:], "  entitys:")
	require.NotEqual(t, -1, entitysIdx, "should have entitys list")

	columnsIdx := strings.Index(config[listsIdx+entitysIdx:], "columns:")
	require.NotEqual(t, -1, columnsIdx, "should have columns")

	createFormIdx := strings.Index(config[listsIdx+entitysIdx+columnsIdx:], "    create_form:")
	require.NotEqual(t, -1, createFormIdx, "should have create_form")

	columnsSection := config[listsIdx+entitysIdx+columnsIdx : listsIdx+entitysIdx+columnsIdx+createFormIdx]

	// Count property occurrences in columns section
	propertyCount := strings.Count(columnsSection, "- property:")
	assert.Equal(t, 4, propertyCount, "should have max 4 columns")
}
