package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// setupSchemaTest sets up the test environment with default metamodel
func setupSchemaTest(t *testing.T) (buf *bytes.Buffer, cleanup func()) {
	t.Helper()

	oldMeta := meta
	oldG := g
	oldOut := out

	meta = metamodel.DefaultMetamodel()
	g = graph.New()

	buf = new(bytes.Buffer)
	out = output.NewWithWriter(buf, output.FormatTable)

	cleanup = func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}

	return buf, cleanup
}

func TestSchemaOverview(t *testing.T) {
	buf, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaCmd.RunE(schemaCmd, []string{})
	if err != nil {
		t.Fatalf("schema overview failed: %v", err)
	}

	result := buf.String()

	// Check for expected content
	if !strings.Contains(result, "Metamodel Overview") {
		t.Errorf("expected 'Metamodel Overview' in output, got: %s", result)
	}
	if !strings.Contains(result, "Entity Types") {
		t.Errorf("expected 'Entity Types' in output, got: %s", result)
	}
	if !strings.Contains(result, "Relation Types") {
		t.Errorf("expected 'Relation Types' in output, got: %s", result)
	}
	if !strings.Contains(result, "Custom Types") {
		t.Errorf("expected 'Custom Types' in output, got: %s", result)
	}

	// Check for specific entity types from default metamodel
	if !strings.Contains(result, "requirement") {
		t.Errorf("expected 'requirement' entity type in output, got: %s", result)
	}
	if !strings.Contains(result, "decision") {
		t.Errorf("expected 'decision' entity type in output, got: %s", result)
	}
}

func TestSchemaOverviewSubcommand(t *testing.T) {
	buf, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaOverviewCmd.RunE(schemaOverviewCmd, []string{})
	if err != nil {
		t.Fatalf("schema overview subcommand failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Metamodel Overview") {
		t.Errorf("expected 'Metamodel Overview' in output, got: %s", result)
	}
}

func TestSchemaEntities(t *testing.T) {
	buf, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaEntitiesCmd.RunE(schemaEntitiesCmd, []string{})
	if err != nil {
		t.Fatalf("schema entities failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Entity Types") {
		t.Errorf("expected 'Entity Types' in output, got: %s", result)
	}

	// Check for entity details
	if !strings.Contains(result, "ID Patterns") {
		t.Errorf("expected 'ID Patterns' in output, got: %s", result)
	}
	if !strings.Contains(result, "Properties") {
		t.Errorf("expected 'Properties' in output, got: %s", result)
	}
	if !strings.Contains(result, "REQ-") {
		t.Errorf("expected 'REQ-' pattern in output, got: %s", result)
	}
}

func TestSchemaRelations(t *testing.T) {
	buf, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaRelationsCmd.RunE(schemaRelationsCmd, []string{})
	if err != nil {
		t.Fatalf("schema relations failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Relation Types") {
		t.Errorf("expected 'Relation Types' in output, got: %s", result)
	}

	// Check for relation details
	if !strings.Contains(result, "addresses") {
		t.Errorf("expected 'addresses' relation in output, got: %s", result)
	}
	if !strings.Contains(result, "From:") {
		t.Errorf("expected 'From:' in output, got: %s", result)
	}
	if !strings.Contains(result, "To:") {
		t.Errorf("expected 'To:' in output, got: %s", result)
	}
}

func TestSchemaTypes(t *testing.T) {
	buf, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaTypesCmd.RunE(schemaTypesCmd, []string{})
	if err != nil {
		t.Fatalf("schema types failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Custom Types") {
		t.Errorf("expected 'Custom Types' in output, got: %s", result)
	}

	// Check for custom types from default metamodel
	if !strings.Contains(result, "status") {
		t.Errorf("expected 'status' type in output, got: %s", result)
	}
	if !strings.Contains(result, "priority") {
		t.Errorf("expected 'priority' type in output, got: %s", result)
	}
	if !strings.Contains(result, "draft") {
		t.Errorf("expected 'draft' value in output, got: %s", result)
	}
}

func TestSchemaEntity(t *testing.T) {
	buf, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaEntityCmd.RunE(schemaEntityCmd, []string{"requirement"})
	if err != nil {
		t.Fatalf("schema entity failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Entity Type: Requirement") {
		t.Errorf("expected 'Entity Type: Requirement' in output, got: %s", result)
	}
	if !strings.Contains(result, "Name: requirement") {
		t.Errorf("expected 'Name: requirement' in output, got: %s", result)
	}
	if !strings.Contains(result, "Aliases:") {
		t.Errorf("expected 'Aliases:' in output, got: %s", result)
	}
	if !strings.Contains(result, "Properties:") {
		t.Errorf("expected 'Properties:' in output, got: %s", result)
	}
	if !strings.Contains(result, "Relations:") {
		t.Errorf("expected 'Relations:' in output, got: %s", result)
	}
}

func TestSchemaEntityWithAlias(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	// Parse the default metamodel from YAML to properly build the alias map
	var err error
	meta, err = metamodel.Parse([]byte(metamodel.DefaultMetamodelYAML()))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	// Test using alias "req" for "requirement"
	err = schemaEntityCmd.RunE(schemaEntityCmd, []string{"req"})
	if err != nil {
		t.Fatalf("schema entity with alias failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Entity Type: Requirement") {
		t.Errorf("expected 'Entity Type: Requirement' in output when using alias, got: %s", result)
	}
}

func TestSchemaEntityNotFound(t *testing.T) {
	_, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaEntityCmd.RunE(schemaEntityCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent entity type")
	}
	if !strings.Contains(err.Error(), "unknown entity type") {
		t.Errorf("expected 'unknown entity type' error, got: %v", err)
	}
}

func TestSchemaRelation(t *testing.T) {
	buf, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaRelationCmd.RunE(schemaRelationCmd, []string{"addresses"})
	if err != nil {
		t.Fatalf("schema relation failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Relation Type: addresses") {
		t.Errorf("expected 'Relation Type: addresses' in output, got: %s", result)
	}
	if !strings.Contains(result, "Name: addresses") {
		t.Errorf("expected 'Name: addresses' in output, got: %s", result)
	}
	if !strings.Contains(result, "From:") {
		t.Errorf("expected 'From:' in output, got: %s", result)
	}
	if !strings.Contains(result, "To:") {
		t.Errorf("expected 'To:' in output, got: %s", result)
	}
	if !strings.Contains(result, "Inverse:") {
		t.Errorf("expected 'Inverse:' in output, got: %s", result)
	}
	if !strings.Contains(result, "Description:") {
		t.Errorf("expected 'Description:' in output, got: %s", result)
	}
}

func TestSchemaRelationNotFound(t *testing.T) {
	_, cleanup := setupSchemaTest(t)
	defer cleanup()

	err := schemaRelationCmd.RunE(schemaRelationCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent relation type")
	}
	if !strings.Contains(err.Error(), "unknown relation type") {
		t.Errorf("expected 'unknown relation type' error, got: %v", err)
	}
}

func TestSchemaOverviewJSON(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	meta = metamodel.DefaultMetamodel()
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)

	err := schemaCmd.RunE(schemaCmd, []string{})
	if err != nil {
		t.Fatalf("schema overview JSON failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected fields
	if _, ok := result["version"]; !ok {
		t.Error("expected 'version' in JSON output")
	}
	if _, ok := result["entities"]; !ok {
		t.Error("expected 'entities' in JSON output")
	}
	if _, ok := result["relations"]; !ok {
		t.Error("expected 'relations' in JSON output")
	}
	if _, ok := result["types"]; !ok {
		t.Error("expected 'types' in JSON output")
	}
}

func TestSchemaEntitiesJSON(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	meta = metamodel.DefaultMetamodel()
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)

	err := schemaEntitiesCmd.RunE(schemaEntitiesCmd, []string{})
	if err != nil {
		t.Fatalf("schema entities JSON failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected entity types
	if _, ok := result["requirement"]; !ok {
		t.Error("expected 'requirement' in JSON output")
	}
	if _, ok := result["decision"]; !ok {
		t.Error("expected 'decision' in JSON output")
	}
}

func TestSchemaEntityJSON(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	meta = metamodel.DefaultMetamodel()
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)

	err := schemaEntityCmd.RunE(schemaEntityCmd, []string{"requirement"})
	if err != nil {
		t.Fatalf("schema entity JSON failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected fields
	if result["name"] != "requirement" {
		t.Errorf("expected name='requirement', got: %v", result["name"])
	}
	if result["label"] != "Requirement" {
		t.Errorf("expected label='Requirement', got: %v", result["label"])
	}
	if _, ok := result["properties"]; !ok {
		t.Error("expected 'properties' in JSON output")
	}
}

func TestSchemaRelationJSON(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	meta = metamodel.DefaultMetamodel()
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)

	err := schemaRelationCmd.RunE(schemaRelationCmd, []string{"addresses"})
	if err != nil {
		t.Fatalf("schema relation JSON failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check for expected fields
	if result["name"] != "addresses" {
		t.Errorf("expected name='addresses', got: %v", result["name"])
	}
	if _, ok := result["from"]; !ok {
		t.Error("expected 'from' in JSON output")
	}
	if _, ok := result["to"]; !ok {
		t.Error("expected 'to' in JSON output")
	}
}

func TestSchemaTypesEmpty(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	// Create metamodel with no custom types
	meta = &metamodel.Metamodel{
		Version:  "1.0",
		Types:    map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	err := schemaTypesCmd.RunE(schemaTypesCmd, []string{})
	if err != nil {
		t.Fatalf("schema types failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "No custom types defined") {
		t.Errorf("expected 'No custom types defined' message, got: %s", result)
	}
}

func TestSchemaEntitiesEmpty(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	// Create metamodel with no entities
	meta = &metamodel.Metamodel{
		Version:   "1.0",
		Types:     map[string]metamodel.CustomType{},
		Entities:  map[string]metamodel.EntityDef{},
		Relations: map[string]metamodel.RelationDef{},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	err := schemaEntitiesCmd.RunE(schemaEntitiesCmd, []string{})
	if err != nil {
		t.Fatalf("schema entities failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "No entity types defined") {
		t.Errorf("expected 'No entity types defined' message, got: %s", result)
	}
}

func TestSchemaRelationsEmpty(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	// Create metamodel with no relations
	meta = &metamodel.Metamodel{
		Version:   "1.0",
		Types:     map[string]metamodel.CustomType{},
		Entities:  map[string]metamodel.EntityDef{},
		Relations: map[string]metamodel.RelationDef{},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	err := schemaRelationsCmd.RunE(schemaRelationsCmd, []string{})
	if err != nil {
		t.Fatalf("schema relations failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "No relation types defined") {
		t.Errorf("expected 'No relation types defined' message, got: %s", result)
	}
}

func TestSchemaWithCardinality(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	sourceMin := 1
	sourceMax := 5

	// Create metamodel with cardinality constraints
	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"entity1": {Label: "Entity1", IDPatterns: []string{"E1-"}},
			"entity2": {Label: "Entity2", IDPatterns: []string{"E2-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"links": {
				Label:     "links",
				From:      []string{"entity1"},
				To:        []string{"entity2"},
				SourceMin: &sourceMin,
				SourceMax: &sourceMax,
			},
		},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	err := schemaRelationsCmd.RunE(schemaRelationsCmd, []string{})
	if err != nil {
		t.Fatalf("schema relations failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Cardinality:") {
		t.Errorf("expected 'Cardinality:' in output, got: %s", result)
	}
	if !strings.Contains(result, "source_min=1") {
		t.Errorf("expected 'source_min=1' in output, got: %s", result)
	}
	if !strings.Contains(result, "source_max=5") {
		t.Errorf("expected 'source_max=5' in output, got: %s", result)
	}
}

func TestSchemaWithSymmetricRelation(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	// Create metamodel with symmetric relation
	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"entity1": {Label: "Entity1", IDPatterns: []string{"E1-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"relates": {
				Label:     "relates to",
				From:      []string{"entity1"},
				To:        []string{"entity1"},
				Symmetric: true,
			},
		},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	err := schemaRelationsCmd.RunE(schemaRelationsCmd, []string{})
	if err != nil {
		t.Fatalf("schema relations failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "Symmetric: yes") {
		t.Errorf("expected 'Symmetric: yes' in output, got: %s", result)
	}
}

func TestSchemaGraphviz(t *testing.T) {
	_, cleanup := setupSchemaTest(t)
	defer cleanup()

	// Reset graphviz flags
	oldGraphviz := schemaGraphviz
	oldConstraints := schemaConstraints
	defer func() {
		schemaGraphviz = oldGraphviz
		schemaConstraints = oldConstraints
	}()

	schemaGraphviz = true
	schemaConstraints = false

	err := runSchemaGraphviz()
	if err != nil {
		t.Fatalf("schema graphviz failed: %v", err)
	}
}

func TestSchemaGraphvizOutput(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	meta = metamodel.DefaultMetamodel()
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	// Capture stdout for runSchemaGraphviz
	oldStdout := captureStdout(t, func() {
		err := runSchemaGraphviz()
		if err != nil {
			t.Fatalf("schema graphviz failed: %v", err)
		}
	})

	// Check DOT format structure
	if !strings.Contains(oldStdout, "digraph metamodel {") {
		t.Errorf("expected 'digraph metamodel {' in output, got: %s", oldStdout)
	}
	if !strings.Contains(oldStdout, "rankdir=LR") {
		t.Errorf("expected 'rankdir=LR' in output, got: %s", oldStdout)
	}
	if !strings.Contains(oldStdout, "// Entity types") {
		t.Errorf("expected '// Entity types' comment in output, got: %s", oldStdout)
	}
	if !strings.Contains(oldStdout, "// Relations") {
		t.Errorf("expected '// Relations' comment in output, got: %s", oldStdout)
	}

	// Check for entity nodes (with colors)
	if !strings.Contains(oldStdout, `requirement [label="Requirement", fillcolor=`) {
		t.Errorf("expected requirement node in output, got: %s", oldStdout)
	}
	if !strings.Contains(oldStdout, `decision [label="Decision", fillcolor=`) {
		t.Errorf("expected decision node in output, got: %s", oldStdout)
	}

	// Check for relation edges (with colors)
	if !strings.Contains(oldStdout, `decision -> requirement [label="addresses"`) {
		t.Errorf("expected addresses edge in output, got: %s", oldStdout)
	}
}

func TestSchemaGraphvizWithConstraints(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	oldConstraints := schemaConstraints
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
		schemaConstraints = oldConstraints
	}()

	sourceMin := 1
	targetMax := 1

	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"source": {Label: "Source", IDPatterns: []string{"SRC-"}},
			"target": {Label: "Target", IDPatterns: []string{"TGT-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"links": {
				Label:     "links to",
				From:      []string{"source"},
				To:        []string{"target"},
				SourceMin: &sourceMin,
				TargetMax: &targetMax,
			},
		},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)
	schemaConstraints = true

	result := captureStdout(t, func() {
		err := runSchemaGraphviz()
		if err != nil {
			t.Fatalf("schema graphviz with constraints failed: %v", err)
		}
	})

	// Check for cardinality in label
	if !strings.Contains(result, "src:1..*") {
		t.Errorf("expected 'src:1..*' cardinality in output, got: %s", result)
	}
	if !strings.Contains(result, "tgt:0..1") {
		t.Errorf("expected 'tgt:0..1' cardinality in output, got: %s", result)
	}
}

func TestSchemaGraphvizWithColors(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"colored": {
				Label:       "Colored Entity",
				IDPatterns:  []string{"COL-"},
				Color:       "#ffcccc",
				BorderColor: "#ff0000",
			},
		},
		Relations: map[string]metamodel.RelationDef{},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	result := captureStdout(t, func() {
		err := runSchemaGraphviz()
		if err != nil {
			t.Fatalf("schema graphviz with colors failed: %v", err)
		}
	})

	// Check for colors in node definition
	if !strings.Contains(result, `fillcolor="#ffcccc"`) {
		t.Errorf("expected fillcolor in output, got: %s", result)
	}
	if !strings.Contains(result, `color="#ff0000"`) {
		t.Errorf("expected border color in output, got: %s", result)
	}
}

func TestSchemaGraphvizMultipleFromTo(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
	}()

	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"a": {Label: "A", IDPatterns: []string{"A-"}},
			"b": {Label: "B", IDPatterns: []string{"B-"}},
			"c": {Label: "C", IDPatterns: []string{"C-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"connects": {
				Label: "connects",
				From:  []string{"a", "b"},
				To:    []string{"b", "c"},
			},
		},
	}
	g = graph.New()

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)

	result := captureStdout(t, func() {
		err := runSchemaGraphviz()
		if err != nil {
			t.Fatalf("schema graphviz with multiple from/to failed: %v", err)
		}
	})

	// Should create edges for all combinations: a->b, a->c, b->b, b->c (with colors)
	if !strings.Contains(result, `a -> b [label="connects"`) {
		t.Errorf("expected a->b edge in output, got: %s", result)
	}
	if !strings.Contains(result, `a -> c [label="connects"`) {
		t.Errorf("expected a->c edge in output, got: %s", result)
	}
	if !strings.Contains(result, `b -> b [label="connects"`) {
		t.Errorf("expected b->b edge in output, got: %s", result)
	}
	if !strings.Contains(result, `b -> c [label="connects"`) {
		t.Errorf("expected b->c edge in output, got: %s", result)
	}
}

func TestFormatCardinality(t *testing.T) {
	tests := []struct {
		name      string
		sourceMin *int
		sourceMax *int
		targetMin *int
		targetMax *int
		expected  string
	}{
		{
			name:     "no constraints",
			expected: "",
		},
		{
			name:      "source min only",
			sourceMin: intPtr(1),
			expected:  "src:1..*",
		},
		{
			name:      "source max only",
			sourceMax: intPtr(5),
			expected:  "src:0..5",
		},
		{
			name:      "source min and max same",
			sourceMin: intPtr(1),
			sourceMax: intPtr(1),
			expected:  "src:1",
		},
		{
			name:      "target min only",
			targetMin: intPtr(1),
			expected:  "tgt:1..*",
		},
		{
			name:      "both source and target",
			sourceMin: intPtr(1),
			targetMax: intPtr(1),
			expected:  "src:1..* tgt:0..1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relDef := metamodel.RelationDef{
				SourceMin: tt.sourceMin,
				SourceMax: tt.sourceMax,
				TargetMin: tt.targetMin,
				TargetMax: tt.targetMax,
			}
			result := formatCardinality(relDef)
			if result != tt.expected {
				t.Errorf("formatCardinality() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}

// captureStdout captures stdout during function execution
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	// Use a pipe to capture stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("failed to read from pipe: %v", err)
	}

	return buf.String()
}
