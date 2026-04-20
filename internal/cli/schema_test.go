package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// setupSchemaTest sets up the test environment with default metamodel
func setupSchemaTest(t *testing.T) (buf *bytes.Buffer, cleanup func()) {
	t.Helper()

	oldMeta := meta
	oldOut := out
	oldWs := ws

	meta = metamodel.DefaultMetamodel()
	applySeeder(newStoreSeeder(meta))

	buf = new(bytes.Buffer)
	out = output.NewWithWriter(buf, output.FormatTable)

	cleanup = func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
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
	if !strings.Contains(result, "ID Prefixes") {
		t.Errorf("expected 'ID Prefixes' in output, got: %s", result)
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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	// Parse the default metamodel from YAML to properly build the alias map
	var err error
	meta, err = metamodel.Parse([]byte(metamodel.DefaultMetamodelYAML()))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = metamodel.DefaultMetamodel()
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = metamodel.DefaultMetamodel()
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = metamodel.DefaultMetamodel()
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = metamodel.DefaultMetamodel()
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	// Create metamodel with no custom types
	meta = &metamodel.Metamodel{
		Version:  "1.0",
		Types:    map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{},
	}
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	// Create metamodel with no entities
	meta = &metamodel.Metamodel{
		Version:   "1.0",
		Types:     map[string]metamodel.CustomType{},
		Entities:  map[string]metamodel.EntityDef{},
		Relations: map[string]metamodel.RelationDef{},
	}
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	// Create metamodel with no relations
	meta = &metamodel.Metamodel{
		Version:   "1.0",
		Types:     map[string]metamodel.CustomType{},
		Entities:  map[string]metamodel.EntityDef{},
		Relations: map[string]metamodel.RelationDef{},
	}
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	sourceMin := 1
	sourceMax := 5

	// Create metamodel with cardinality constraints
	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"entity1": {Label: "Entity1", IDPrefix: "E1-"},
			"entity2": {Label: "Entity2", IDPrefix: "E2-"},
		},
		Relations: map[string]metamodel.RelationDef{
			"links": {
				Label:       "links",
				From:        []string{"entity1"},
				To:          []string{"entity2"},
				MinOutgoing: &sourceMin,
				MaxOutgoing: &sourceMax,
			},
		},
	}
	applySeeder(newStoreSeeder(meta))

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
	if !strings.Contains(result, "min_outgoing=1") {
		t.Errorf("expected 'min_outgoing=1' in output, got: %s", result)
	}
	if !strings.Contains(result, "max_outgoing=5") {
		t.Errorf("expected 'max_outgoing=5' in output, got: %s", result)
	}
}

func TestSchemaWithSymmetricRelation(t *testing.T) {
	oldMeta := meta
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	// Create metamodel with symmetric relation
	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"entity1": {Label: "Entity1", IDPrefix: "E1-"},
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
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = metamodel.DefaultMetamodel()
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	oldConstraints := schemaConstraints
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
		schemaConstraints = oldConstraints
	}()

	sourceMin := 1
	targetMax := 1

	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"source": {Label: "Source", IDPrefix: "SRC-"},
			"target": {Label: "Target", IDPrefix: "TGT-"},
		},
		Relations: map[string]metamodel.RelationDef{
			"links": {
				Label:       "links to",
				From:        []string{"source"},
				To:          []string{"target"},
				MinOutgoing: &sourceMin,
				MaxIncoming: &targetMax,
			},
		},
	}
	applySeeder(newStoreSeeder(meta))

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
	if !strings.Contains(result, "out:1..*") {
		t.Errorf("expected 'out:1..*' cardinality in output, got: %s", result)
	}
	if !strings.Contains(result, "in:0..1") {
		t.Errorf("expected 'in:0..1' cardinality in output, got: %s", result)
	}
}

func TestSchemaGraphvizWithColors(t *testing.T) {
	oldMeta := meta
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"colored": {
				Label:       "Colored Entity",
				IDPrefix:    "COL-",
				Color:       "#ffcccc",
				BorderColor: "#ff0000",
			},
		},
		Relations: map[string]metamodel.RelationDef{},
	}
	applySeeder(newStoreSeeder(meta))

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
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = &metamodel.Metamodel{
		Version: "1.0",
		Types:   map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"a": {Label: "A", IDPrefix: "A-"},
			"b": {Label: "B", IDPrefix: "B-"},
			"c": {Label: "C", IDPrefix: "C-"},
		},
		Relations: map[string]metamodel.RelationDef{
			"connects": {
				Label: "connects",
				From:  []string{"a", "b"},
				To:    []string{"b", "c"},
			},
		},
	}
	applySeeder(newStoreSeeder(meta))

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
		name        string
		minOutgoing *int
		maxOutgoing *int
		minIncoming *int
		maxIncoming *int
		expected    string
	}{
		{
			name:     "no constraints",
			expected: "",
		},
		{
			name:        "min outgoing only",
			minOutgoing: intPtr(1),
			expected:    "out:1..*",
		},
		{
			name:        "max outgoing only",
			maxOutgoing: intPtr(5),
			expected:    "out:0..5",
		},
		{
			name:        "min and max outgoing same",
			minOutgoing: intPtr(1),
			maxOutgoing: intPtr(1),
			expected:    "out:1",
		},
		{
			name:        "min incoming only",
			minIncoming: intPtr(1),
			expected:    "in:1..*",
		},
		{
			name:        "both outgoing and incoming",
			minOutgoing: intPtr(1),
			maxIncoming: intPtr(1),
			expected:    "out:1..* in:0..1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relDef := metamodel.RelationDef{
				MinOutgoing: tt.minOutgoing,
				MaxOutgoing: tt.maxOutgoing,
				MinIncoming: tt.minIncoming,
				MaxIncoming: tt.maxIncoming,
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

// --- graphviz rendering-pipeline tests -------------------------------------
//
// These cover the --exclude flag and the classification rule that renders
// many-target relations as a hub bundle or a legend table rather than a
// fan of edges.

// schemaGraphvizFixture builds a bare metamodel with the given entities and
// relations, applies it, and restores the previous state on cleanup.
// Pass bs as nil to keep all rendering flags at their default.
func schemaGraphvizFixture(
	t *testing.T,
	ents map[string]metamodel.EntityDef,
	rels map[string]metamodel.RelationDef,
) {
	t.Helper()
	oldMeta := meta
	oldOut := out
	oldWs := ws
	oldGraphviz := schemaGraphviz
	oldExclude := schemaExclude
	oldNoBundle := schemaNoBundle
	oldNoLegend := schemaNoLegend
	t.Cleanup(func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
		schemaGraphviz = oldGraphviz
		schemaExclude = oldExclude
		schemaNoBundle = oldNoBundle
		schemaNoLegend = oldNoLegend
	})

	meta = &metamodel.Metamodel{
		Version:   "1.0",
		Types:     map[string]metamodel.CustomType{},
		Entities:  ents,
		Relations: rels,
	}
	applySeeder(newStoreSeeder(meta))

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatTable)
	schemaGraphviz = true
	schemaExclude = nil
	schemaNoBundle = false
	schemaNoLegend = false
}

func TestSchemaGraphvizExclude(t *testing.T) {
	schemaGraphvizFixture(t,
		map[string]metamodel.EntityDef{
			"a": {Label: "A", IDPrefix: "A-"},
			"b": {Label: "B", IDPrefix: "B-"},
			"c": {Label: "C", IDPrefix: "C-"},
		},
		map[string]metamodel.RelationDef{
			"ab": {Label: "ab", From: []string{"a"}, To: []string{"b"}},
			"ac": {Label: "ac", From: []string{"a"}, To: []string{"c"}},
		},
	)
	schemaExclude = []string{"c"}

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if strings.Contains(result, "  c [label") {
		t.Errorf("excluded node c should not appear:\n%s", result)
	}
	if strings.Contains(result, `a -> c `) || strings.Contains(result, `-> c [`) {
		t.Errorf("edges to c should not appear:\n%s", result)
	}
	if !strings.Contains(result, `a -> b `) {
		t.Errorf("non-excluded edge a->b should appear:\n%s", result)
	}
}

func TestSchemaGraphvizLegendFiveTargets(t *testing.T) {
	ents := map[string]metamodel.EntityDef{
		"src": {Label: "Src", IDPrefix: "S-"},
	}
	// 5 unconnected leaf targets -> relation with 5 targets -> legend bucket.
	targets := []string{"t1", "t2", "t3", "t4", "t5"}
	for _, t := range targets {
		ents[t] = metamodel.EntityDef{Label: strings.ToUpper(t), IDPrefix: t + "-"}
	}
	schemaGraphvizFixture(t, ents,
		map[string]metamodel.RelationDef{
			"fans": {Label: "fans", From: []string{"src"}, To: targets},
		},
	)

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if !strings.Contains(result, "__legend [") {
		t.Errorf("expected __legend node:\n%s", result)
	}
	if strings.Contains(result, `src -> t1`) || strings.Contains(result, `src -> t5`) {
		t.Errorf("5-target relation should not emit direct edges:\n%s", result)
	}
	if !strings.Contains(result, "<B>Src</B> <I>fans</I>") {
		t.Errorf("legend should contain source + relation label:\n%s", result)
	}
}

func TestSchemaGraphvizHubIsolatedTargets(t *testing.T) {
	ents := map[string]metamodel.EntityDef{
		"src": {Label: "Src", IDPrefix: "S-"},
		"t1":  {Label: "T1", IDPrefix: "T1-"},
		"t2":  {Label: "T2", IDPrefix: "T2-"},
		"t3":  {Label: "T3", IDPrefix: "T3-"},
	}
	// 3 isolated targets -> hub bundle.
	schemaGraphvizFixture(t, ents,
		map[string]metamodel.RelationDef{
			"fans": {Label: "fans", From: []string{"src"}, To: []string{"t1", "t2", "t3"}},
		},
	)

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if !strings.Contains(result, "__hub_0 [shape=point") {
		t.Errorf("expected __hub_0 point node:\n%s", result)
	}
	if !strings.Contains(result, `src -> __hub_0 [label="fans"`) {
		t.Errorf("expected labeled source->hub edge:\n%s", result)
	}
	if !strings.Contains(result, `__hub_0 -> t1 [color=`) {
		t.Errorf("expected hub->target edges (unlabeled):\n%s", result)
	}
	if strings.Contains(result, "__legend [") {
		t.Errorf("3 isolated targets should use hub, not legend:\n%s", result)
	}
}

func TestSchemaGraphvizLegendConnectedTargets(t *testing.T) {
	// 4 targets, each with another incoming edge -> all otherwise-connected
	// -> legend (not hub).
	ents := map[string]metamodel.EntityDef{
		"src":    {Label: "Src", IDPrefix: "S-"},
		"anchor": {Label: "Anchor", IDPrefix: "A-"},
		"t1":     {Label: "T1", IDPrefix: "T1-"},
		"t2":     {Label: "T2", IDPrefix: "T2-"},
		"t3":     {Label: "T3", IDPrefix: "T3-"},
		"t4":     {Label: "T4", IDPrefix: "T4-"},
	}
	schemaGraphvizFixture(t, ents,
		map[string]metamodel.RelationDef{
			"fans": {Label: "fans", From: []string{"src"}, To: []string{"t1", "t2", "t3", "t4"}},
			// Each target additionally has an incoming edge from anchor,
			// giving every target "otherwise connected" status.
			"anchors": {Label: "anchors", From: []string{"anchor"}, To: []string{"t1", "t2", "t3", "t4"}},
		},
	)

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if !strings.Contains(result, "__legend [") {
		t.Errorf("4 connected targets should collapse to legend:\n%s", result)
	}
	if strings.Contains(result, "__hub_") {
		t.Errorf("should not emit hub when all targets are otherwise connected:\n%s", result)
	}
	// anchors relation also has 4 targets all connected -> also legend,
	// but self-check: no direct src->tN edges for the 'fans' relation.
	if strings.Contains(result, `src -> t1 [label="fans"`) {
		t.Errorf("fans edges should be suppressed:\n%s", result)
	}
}

func TestSchemaGraphvizFewTargetsPlain(t *testing.T) {
	schemaGraphvizFixture(t,
		map[string]metamodel.EntityDef{
			"a": {Label: "A", IDPrefix: "A-"},
			"b": {Label: "B", IDPrefix: "B-"},
			"c": {Label: "C", IDPrefix: "C-"},
		},
		map[string]metamodel.RelationDef{
			"r": {Label: "r", From: []string{"a"}, To: []string{"b", "c"}},
		},
	)

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if !strings.Contains(result, `a -> b [label="r"`) ||
		!strings.Contains(result, `a -> c [label="r"`) {

		t.Errorf("2-target relation should emit plain edges:\n%s", result)
	}
	if strings.Contains(result, "__hub_") || strings.Contains(result, "__legend [") {
		t.Errorf("≤2 targets should never hub/legend:\n%s", result)
	}
}

func TestSchemaGraphvizDropsEmptyNode(t *testing.T) {
	// 'island' participates only in a legend-collapsed relation (≥5 targets),
	// and has no other edges, so it must be dropped from the body.
	ents := map[string]metamodel.EntityDef{
		"island": {Label: "Island", IDPrefix: "I-"},
	}
	targets := []string{"t1", "t2", "t3", "t4", "t5"}
	for _, t := range targets {
		ents[t] = metamodel.EntityDef{Label: strings.ToUpper(t), IDPrefix: t + "-"}
	}
	schemaGraphvizFixture(t, ents,
		map[string]metamodel.RelationDef{
			"fans": {Label: "fans", From: []string{"island"}, To: targets},
			// each target has another connection so they stay visible.
			"link": {Label: "link", From: []string{"t1"}, To: []string{"t2"}},
		},
	)

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if strings.Contains(result, `island [label`) {
		t.Errorf("legend-only entity 'island' should be hidden:\n%s", result)
	}
	if !strings.Contains(result, `t1 [label="T1"`) {
		t.Errorf("connected target t1 should remain visible:\n%s", result)
	}
}

func TestSchemaGraphvizNoLegendFlag(t *testing.T) {
	ents := map[string]metamodel.EntityDef{
		"src": {Label: "Src", IDPrefix: "S-"},
	}
	targets := []string{"t1", "t2", "t3", "t4", "t5"}
	for _, t := range targets {
		ents[t] = metamodel.EntityDef{Label: strings.ToUpper(t), IDPrefix: t + "-"}
	}
	schemaGraphvizFixture(t, ents,
		map[string]metamodel.RelationDef{
			"fans": {Label: "fans", From: []string{"src"}, To: targets},
		},
	)
	schemaNoLegend = true

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if strings.Contains(result, "__legend [") {
		t.Errorf("--no-legend should suppress the legend:\n%s", result)
	}
}

func TestSchemaGraphvizNoBundleFlag(t *testing.T) {
	ents := map[string]metamodel.EntityDef{
		"src": {Label: "Src", IDPrefix: "S-"},
		"t1":  {Label: "T1", IDPrefix: "T1-"},
		"t2":  {Label: "T2", IDPrefix: "T2-"},
		"t3":  {Label: "T3", IDPrefix: "T3-"},
	}
	schemaGraphvizFixture(t, ents,
		map[string]metamodel.RelationDef{
			"fans": {Label: "fans", From: []string{"src"}, To: []string{"t1", "t2", "t3"}},
		},
	)
	schemaNoBundle = true

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	if strings.Contains(result, "__hub_") {
		t.Errorf("--no-bundle should suppress hub even when targets are isolated:\n%s", result)
	}
	// With no bundle and isolated targets, the classification falls back to
	// legend — which is the correct remaining collapse path.
	if !strings.Contains(result, "__legend [") {
		t.Errorf("with --no-bundle the pair should fall back to legend:\n%s", result)
	}
}

func TestFormatTargets(t *testing.T) {
	labels := map[string]string{
		"a": "A", "b": "B", "c": "C", "d": "D", "e": "E",
	}
	tests := []struct {
		name    string
		to      []string
		total   int
		want    string
		wantSub string
	}{
		{name: "empty total returns empty", total: 0, want: ""},
		{name: "exactly all", to: []string{"a", "b", "c", "d", "e"}, total: 5, want: "any entity"},
		{name: "minus one", to: []string{"a", "c", "d", "e"}, total: 5, wantSub: "except B"},
		{name: "minus two", to: []string{"c", "d", "e"}, total: 5, wantSub: "except A, B"},
		{name: "small list", to: []string{"c", "a"}, total: 5, wantSub: "A, C"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatTargets(tc.to, labels, tc.total)
			if tc.want != "" && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			if tc.wantSub != "" && !strings.Contains(got, tc.wantSub) {
				t.Errorf("got %q, want substring %q", got, tc.wantSub)
			}
		})
	}
}

func TestSchemaGraphvizEscapesHTML(t *testing.T) {
	ents := map[string]metamodel.EntityDef{
		"src": {Label: "Src<&>", IDPrefix: "S-"},
	}
	targets := []string{"t1", "t2", "t3", "t4", "t5"}
	for _, t := range targets {
		ents[t] = metamodel.EntityDef{Label: strings.ToUpper(t), IDPrefix: t + "-"}
	}
	schemaGraphvizFixture(t, ents,
		map[string]metamodel.RelationDef{
			"rel": {Label: `has "quote"`, From: []string{"src"}, To: targets},
		},
	)

	result := captureStdout(t, func() {
		if err := runSchemaGraphviz(); err != nil {
			t.Fatalf("runSchemaGraphviz: %v", err)
		}
	})

	// The legend uses HTML-like labels, so the unsafe characters must be escaped.
	if !strings.Contains(result, "Src&lt;&amp;&gt;") {
		t.Errorf("legend should HTML-escape source label:\n%s", result)
	}
	if !strings.Contains(result, "has &#34;quote&#34;") {
		t.Errorf("legend should HTML-escape relation label:\n%s", result)
	}
}
