package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Mock types for schema JSON testing
type mockSchemaMetamodel struct{}

func (m *mockSchemaMetamodel) GetVersion() string   { return "1.0" }
func (m *mockSchemaMetamodel) GetNamespace() string { return "test" }
func (m *mockSchemaMetamodel) GetEntities() any     { return map[string]any{} }
func (m *mockSchemaMetamodel) GetRelations() any    { return map[string]any{} }
func (m *mockSchemaMetamodel) GetTypes() any        { return map[string]any{} }

type mockSchemaEntityDef struct{}

func (e *mockSchemaEntityDef) GetLabel() string        { return "Test Entity" }
func (e *mockSchemaEntityDef) GetAliases() []string    { return []string{"test"} }
func (e *mockSchemaEntityDef) GetIDPatterns() []string { return []string{"TEST-*"} }
func (e *mockSchemaEntityDef) GetProperties() any      { return map[string]any{} }
func (e *mockSchemaEntityDef) GetRDFType() string      { return "test:Entity" }
func (e *mockSchemaEntityDef) GetColor() string        { return "#FF0000" }
func (e *mockSchemaEntityDef) GetBorderColor() string  { return "#000000" }

type mockSchemaRelationDef struct {
	symmetric bool
	desc      string
	inverse   any
	srcMin    *int
	srcMax    *int
	tgtMin    *int
	tgtMax    *int
}

func (r *mockSchemaRelationDef) GetLabel() string  { return "Test Relation" }
func (r *mockSchemaRelationDef) GetFrom() []string { return []string{"Entity1"} }
func (r *mockSchemaRelationDef) GetTo() []string   { return []string{"Entity2"} }
func (r *mockSchemaRelationDef) GetDescription() string {
	return r.desc
}
func (r *mockSchemaRelationDef) GetInverse() any      { return r.inverse }
func (r *mockSchemaRelationDef) IsSymmetric() bool    { return r.symmetric }
func (r *mockSchemaRelationDef) GetMinOutgoing() *int { return r.srcMin }
func (r *mockSchemaRelationDef) GetMaxOutgoing() *int { return r.srcMax }
func (r *mockSchemaRelationDef) GetMinIncoming() *int { return r.tgtMin }
func (r *mockSchemaRelationDef) GetMaxIncoming() *int { return r.tgtMax }

// TestSchemaJSONOverview tests schema overview output
func TestSchemaJSONOverview(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	mock := &mockSchemaMetamodel{}
	if err := w.writeOverview(mock); err != nil {
		t.Fatalf("writeOverview failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result["version"] != "1.0" {
		t.Error("expected version in output")
	}
}

// TestSchemaJSONEntities tests schema entities output
func TestSchemaJSONEntities(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	mock := &mockSchemaMetamodel{}
	if err := w.writeEntities(mock); err != nil {
		t.Fatalf("writeEntities failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

// TestSchemaJSONRelations tests schema relations output
func TestSchemaJSONRelations(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	mock := &mockSchemaMetamodel{}
	if err := w.writeRelations(mock); err != nil {
		t.Fatalf("writeRelations failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

// TestSchemaJSONTypes tests schema types output
func TestSchemaJSONTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	mock := &mockSchemaMetamodel{}
	if err := w.writeTypes(mock); err != nil {
		t.Fatalf("writeTypes failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

// TestSchemaJSONEntityDetail tests entity detail output
func TestSchemaJSONEntityDetail(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	mockDef := &mockSchemaEntityDef{}
	if err := w.writeEntityDetail("test", mockDef); err != nil {
		t.Fatalf("writeEntityDetail failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result["name"] != "test" {
		t.Error("expected name in output")
	}
}

// TestSchemaJSONRelationDetail tests relation detail output
func TestSchemaJSONRelationDetail(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	mockDef := &mockSchemaRelationDef{}
	if err := w.writeRelationDetail("test_relation", mockDef); err != nil {
		t.Fatalf("writeRelationDetail failed: %v", err)
	}

	var result map[string]any
	if unmarshalErr := json.Unmarshal(buf.Bytes(), &result); unmarshalErr != nil {
		t.Fatalf("failed to parse JSON output: %v", unmarshalErr)
	}
	if result["name"] != "test_relation" {
		t.Error("expected name in output")
	}

	// Test with all optional fields
	buf2 := &bytes.Buffer{}
	w2 := schemaJSONWriter{out: buf2}

	srcMin := 1
	srcMax := 5
	tgtMin := 2
	tgtMax := 10
	mockDef2 := &mockSchemaRelationDef{
		desc:      "Test description",
		inverse:   "test_inverse",
		symmetric: true,
		srcMin:    &srcMin,
		srcMax:    &srcMax,
		tgtMin:    &tgtMin,
		tgtMax:    &tgtMax,
	}
	if err := w2.writeRelationDetail("test_relation2", mockDef2); err != nil {
		t.Fatalf("writeRelationDetail with optional fields failed: %v", err)
	}

	var result2 map[string]any
	if err := json.Unmarshal(buf2.Bytes(), &result2); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if result2["description"] != "Test description" {
		t.Error("expected description in output")
	}
	if result2["symmetric"] != true {
		t.Error("expected symmetric in output")
	}
	if result2["min_outgoing"] != float64(1) {
		t.Error("expected min_outgoing in output")
	}
}
