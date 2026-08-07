package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// decodeConfig parses the string returned by generateDataEntryConfig into a
// generic map. Tests use this rather than substring matching so they stay
// independent of yaml.v3 formatting choices (when it quotes vs doesn't).
func decodeConfig(t *testing.T, config string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(config), &out), "generated config must be valid YAML")
	return out
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
				Label: "Feature",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: "string"},
					"status":   {Type: "string"},
					"priority": {Type: "string"},
				},
			},
			"bug": {
				Label: "Bug",
				Properties: map[string]metamodel.PropertyDef{
					"title":       {Type: "string"},
					"description": {Type: "text"},
				},
			},
		},
	}

	cfg := decodeConfig(t, generateDataEntryConfig("Test App", meta))

	app := cfg["app"].(map[string]any)
	assert.Equal(t, "Test App", app["name"])

	forms := cfg["forms"].(map[string]any)
	feature := forms["feature"].(map[string]any)
	assert.Equal(t, "feature", feature["entity_type"])
	assert.Equal(t, "Feature", feature["title"])
	bug := forms["bug"].(map[string]any)
	assert.Equal(t, "bug", bug["entity_type"])
	assert.Equal(t, "Bug", bug["title"])

	lists := cfg["lists"].(map[string]any)
	featuresList := lists["features"].(map[string]any)
	assert.Equal(t, "feature", featuresList["entity_type"])
	bugsList := lists["bugs"].(map[string]any)
	assert.Equal(t, "bug", bugsList["entity_type"])

	navigation := cfg["navigation"].([]any)
	navLabels := make([]string, 0, len(navigation))
	for _, item := range navigation {
		navLabels = append(navLabels, item.(map[string]any)["label"].(string))
	}
	assert.Contains(t, navLabels, "Features")
	assert.Contains(t, navLabels, "Bugs")

	// Properties are sorted alphabetically
	featureFields := feature["fields"].([]any)
	propNames := make([]string, 0, len(featureFields))
	for _, f := range featureFields {
		propNames = append(propNames, f.(map[string]any)["property"].(string))
	}
	assert.Equal(t, []string{"priority", "status", "title"}, propNames)
}

func TestGenerateDataEntryConfig_EmptyMetamodel(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{},
	}

	cfg := decodeConfig(t, generateDataEntryConfig("Empty App", meta))
	app := cfg["app"].(map[string]any)
	assert.Equal(t, "Empty App", app["name"])
	assert.Contains(t, cfg, "forms")
	assert.Contains(t, cfg, "lists")
	assert.Contains(t, cfg, "navigation")
}

func TestGenerateDataEntryConfig_KebabCaseEntityType(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"test-case": {
				Label: "Test Case",
				Properties: map[string]metamodel.PropertyDef{
					"test-name": {Type: "string"},
				},
			},
		},
	}

	cfg := decodeConfig(t, generateDataEntryConfig("Test App", meta))

	forms := cfg["forms"].(map[string]any)
	testCase := forms["test_case"].(map[string]any) // hyphen -> underscore for the key
	assert.Equal(t, "test-case", testCase["entity_type"])
	// The title comes from the AUTHORED metamodel label, not from
	// titleCase(typeName) — DEC-6C1NAA.
	assert.Equal(t, "Test Case", testCase["title"])

	lists := cfg["lists"].(map[string]any)
	require.Contains(t, lists, "test_cases")

	navigation := cfg["navigation"].([]any)
	require.Len(t, navigation, 1)
	nav := navigation[0].(map[string]any)
	assert.Equal(t, "Test Cases", nav["label"])
	assert.Equal(t, "test_cases", nav["list"])
}

// TestGenerateDataEntryConfig_NoDerivedFieldLabels pins DEC-6C1NAA for the
// scaffolder: generated fields and columns carry no `label:` key at all, so a
// new project never inherits an English orthographic guess. The user authors
// each label in their own language.
func TestGenerateDataEntryConfig_NoDerivedFieldLabels(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"due_date": {Type: "date"},
				},
			},
		},
	}

	raw := generateDataEntryConfig("Test App", meta)
	assert.NotContains(t, raw, "Due Date",
		"scaffolder must not emit a title-cased label for due_date")

	cfg := decodeConfig(t, raw)

	forms := cfg["forms"].(map[string]any)
	fields := forms["ticket"].(map[string]any)["fields"].([]any)
	require.Len(t, fields, 1)
	field := fields[0].(map[string]any)
	assert.Equal(t, "due_date", field["property"])
	assert.NotContains(t, field, "label", "generated fields carry no label key")

	lists := cfg["lists"].(map[string]any)
	columns := lists["tickets"].(map[string]any)["columns"].([]any)
	require.Len(t, columns, 1)
	assert.NotContains(t, columns[0].(map[string]any), "label",
		"generated columns carry no label key")
}

// TestGenerateDataEntryConfig_FallsBackToTypeNameWithoutLabel covers the
// in-memory-metamodel path: the loader requires entity labels, but this
// generator can be handed a metamodel built in code. A missing label must
// yield the raw type name, never an empty title or a bare "s" plural.
func TestGenerateDataEntryConfig_FallsBackToTypeNameWithoutLabel(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
		},
	}

	cfg := decodeConfig(t, generateDataEntryConfig("Test App", meta))

	assert.Equal(t, "ticket", cfg["forms"].(map[string]any)["ticket"].(map[string]any)["title"])
	assert.Equal(t, "tickets", cfg["lists"].(map[string]any)["tickets"].(map[string]any)["title"])
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

	cfg := decodeConfig(t, generateDataEntryConfig("Test App", meta))
	entitys := cfg["lists"].(map[string]any)["entitys"].(map[string]any)
	columns := entitys["columns"].([]any)
	assert.Len(t, columns, 4, "lists should cap at 4 columns")
}

// TestGenerateDataEntryConfig_YAMLSpecialChars is the regression test for
// BUG-F9I2Z: titles derived from entity/property names that contain YAML-special
// characters must still produce valid YAML. The previous Fprintf-based
// implementation would embed the raw string inside double quotes without
// escaping, producing unparseable output for names like `foo"bar` or `a\nb`.
func TestGenerateDataEntryConfig_YAMLSpecialChars(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			`quote"type`: {
				Properties: map[string]metamodel.PropertyDef{
					`back\slash`:    {Type: "string"},
					"newline\nprop": {Type: "string"},
					"tab\tprop":     {Type: "string"},
				},
			},
		},
	}

	appName := `App with "quotes" and \backslash`
	cfg := decodeConfig(t, generateDataEntryConfig(appName, meta))

	// Round-tripping proves the YAML parsed; spot-check that values survived
	// unmangled.
	assert.Equal(t, appName, cfg["app"].(map[string]any)["name"])

	forms := cfg["forms"].(map[string]any)
	// The form key goes through hyphen→underscore, but the entity_type value is
	// the raw string. Exactly one form was generated; find it.
	require.Len(t, forms, 1)
	for _, v := range forms {
		form := v.(map[string]any)
		assert.Equal(t, `quote"type`, form["entity_type"])
		fields := form["fields"].([]any)
		propNames := make([]string, 0, len(fields))
		for _, f := range fields {
			propNames = append(propNames, f.(map[string]any)["property"].(string))
		}
		// Sorted alphabetically by the generator.
		assert.ElementsMatch(t, []string{`back\slash`, "newline\nprop", "tab\tprop"}, propNames)
	}
}
