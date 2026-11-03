package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// testSchemaMetamodel returns a minimal metamodel for schema JSON tests.
func testSchemaMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version:   "1.0",
		Namespace: "test",
		Entities:  map[string]metamodel.EntityDef{},
		Relations: map[string]metamodel.RelationDef{},
		Types:     map[string]metamodel.CustomType{},
	}
}

// TestSchemaJSONOverview tests schema overview output
func TestSchemaJSONOverview(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	meta := testSchemaMetamodel()
	if err := w.writeOverview(meta); err != nil {
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

	meta := testSchemaMetamodel()
	if err := w.writeEntities(meta); err != nil {
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

	meta := testSchemaMetamodel()
	if err := w.writeRelations(meta); err != nil {
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

	meta := testSchemaMetamodel()
	if err := w.writeTypes(meta); err != nil {
		t.Fatalf("writeTypes failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

// assertJSONDocument compares written JSON with want, ignoring key order.
func assertJSONDocument(t *testing.T, got []byte, want string) {
	t.Helper()
	var gotDoc, wantDoc any
	if err := json.Unmarshal(got, &gotDoc); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantDoc); err != nil {
		t.Fatalf("parse want: %v", err)
	}
	if !reflect.DeepEqual(gotDoc, wantDoc) {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

// TestSchemaJSONEntityDetail pins every field of the entity detail output.
func TestSchemaJSONEntityDetail(t *testing.T) {
	buf := &bytes.Buffer{}
	w := schemaJSONWriter{out: buf}

	def := &metamodel.EntityDef{
		Label:       "Test Entity",
		Aliases:     []string{"test"},
		IDPrefix:    "TEST-",
		RDFType:     "test:Entity",
		Color:       "#FF0000",
		BorderColor: "#000000",
	}
	if err := w.writeEntityDetail("test", def); err != nil {
		t.Fatalf("writeEntityDetail failed: %v", err)
	}

	assertJSONDocument(t, buf.Bytes(), `{
		"name": "test", "label": "Test Entity", "aliases": ["test"],
		"id_patterns": ["TEST-"], "properties": null,
		"rdf_type": "test:Entity", "color": "#FF0000", "border_color": "#000000"
	}`)
}

// TestSchemaJSONRelationDetail pins the relation detail output with and
// without the optional fields.
func TestSchemaJSONRelationDetail(t *testing.T) {
	srcMin, srcMax, tgtMin, tgtMax := 1, 5, 2, 10
	tests := []struct {
		name string
		def  *metamodel.RelationDef
		want string
	}{
		{
			name: "required fields only",
			def:  &metamodel.RelationDef{Label: "Test Relation", From: []string{"Entity1"}, To: []string{"Entity2"}},
			want: `{"name": "test_relation", "label": "Test Relation", "from": ["Entity1"], "to": ["Entity2"]}`,
		},
		{
			name: "all optional fields",
			def: &metamodel.RelationDef{
				Label:       "Test Relation",
				From:        []string{"Entity1"},
				To:          []string{"Entity2"},
				Description: "Test description",
				Inverse:     &metamodel.InverseDef{ID: "test_inverse", Label: "inverse of test"},
				Symmetric:   true,
				Owning:      true,
				MinOutgoing: &srcMin,
				MaxOutgoing: &srcMax,
				MinIncoming: &tgtMin,
				MaxIncoming: &tgtMax,
			},
			want: `{
				"name": "test_relation", "label": "Test Relation", "from": ["Entity1"], "to": ["Entity2"],
				"description": "Test description",
				"inverse": {"ID": "test_inverse", "Label": "inverse of test"},
				"symmetric": true, "owning": true,
				"min_outgoing": 1, "max_outgoing": 5, "min_incoming": 2, "max_incoming": 10
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := schemaJSONWriter{out: buf}
			if err := w.writeRelationDetail("test_relation", tc.def); err != nil {
				t.Fatalf("writeRelationDetail failed: %v", err)
			}
			assertJSONDocument(t, buf.Bytes(), tc.want)
		})
	}
}
