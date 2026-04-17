package importer

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// testMetamodel creates a test metamodel
func testMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Types: map[string]metamodel.CustomType{
			"status": {
				Values:  []string{"draft", "approved", "accepted", "rejected"},
				Default: "draft",
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
					"status": {
						Type:   "status",
						Values: []string{"draft", "approved", "rejected"},
					},
					"priority": {
						Type:   "enum",
						Values: []string{"low", "medium", "high"},
					},
				},
			},
			"decision": {
				Label:    "Decision",
				IDPrefix: "DEC-",
				Properties: map[string]metamodel.PropertyDef{
					"title": {
						Type:     "string",
						Required: true,
					},
					"status": {
						Type:   "status",
						Values: []string{"draft", "accepted", "rejected"},
					},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"addresses": {
				Label: "Addresses",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
		},
	}
}

func newTestStore() store.Store {
	return memstore.New()
}

func newTestSource() *ImportSource {
	return NewImportSource(storage.NewMemFS())
}

func ctx() context.Context { return context.Background() }

func TestParseJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEnts int
		wantRels int
		wantErr  bool
	}{
		{
			name: "full format",
			input: `{
				"entities": [
					{"id": "REQ-001", "type": "requirement", "properties": {"title": "Test"}}
				],
				"relations": [
					{"from": "DEC-001", "relation": "addresses", "to": "REQ-001"}
				]
			}`,
			wantEnts: 1,
			wantRels: 1,
		},
		{
			name: "array format",
			input: `[
				{"id": "REQ-001", "type": "requirement", "properties": {"title": "Test 1"}},
				{"id": "REQ-002", "type": "requirement", "properties": {"title": "Test 2"}}
			]`,
			wantEnts: 2,
			wantRels: 0,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   `"just a string"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := parseJSON(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(data.Entities) != tt.wantEnts {
				t.Errorf("parseJSON() entities = %d, want %d", len(data.Entities), tt.wantEnts)
			}
			if len(data.Relations) != tt.wantRels {
				t.Errorf("parseJSON() relations = %d, want %d", len(data.Relations), tt.wantRels)
			}
		})
	}
}

func TestParseYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEnts int
		wantRels int
		wantErr  bool
	}{
		{
			name: "full format",
			input: `
entities:
  - id: REQ-001
    type: requirement
    properties:
      title: Test
relations:
  - from: DEC-001
    relation: addresses
    to: REQ-001
`,
			wantEnts: 1,
			wantRels: 1,
		},
		{
			name: "array format",
			input: `
- id: REQ-001
  type: requirement
  properties:
    title: Test 1
- id: REQ-002
  type: requirement
  properties:
    title: Test 2
`,
			wantEnts: 2,
			wantRels: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := parseYAML(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(data.Entities) != tt.wantEnts {
				t.Errorf("parseYAML() entities = %d, want %d", len(data.Entities), tt.wantEnts)
			}
			if len(data.Relations) != tt.wantRels {
				t.Errorf("parseYAML() relations = %d, want %d", len(data.Relations), tt.wantRels)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEnts int
		wantErr  bool
	}{
		{
			name: "basic csv",
			input: `id,type,title,status
REQ-001,requirement,Test requirement,draft
REQ-002,requirement,Another requirement,approved
`,
			wantEnts: 2,
		},
		{
			name:    "missing id column",
			input:   "type,title\nrequirement,Test\n",
			wantErr: true,
		},
		{
			name:    "missing type column",
			input:   "id,title\nREQ-001,Test\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := parseCSV(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCSV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if len(data.Entities) != tt.wantEnts {
				t.Errorf("parseCSV() entities = %d, want %d", len(data.Entities), tt.wantEnts)
			}
		})
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path string
		want Format
	}{
		{"data.json", FormatJSON},
		{"data.JSON", FormatJSON},
		{"data.yaml", FormatYAML},
		{"data.yml", FormatYAML},
		{"data.csv", FormatCSV},
		{"data.txt", ""},
		{"data", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectFormat(tt.path)
			if got != tt.want {
				t.Errorf("detectFormat(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestImportDryRun(t *testing.T) {
	st := newTestStore()
	meta := testMetamodel()

	imp := New(st, meta, Options{DryRun: true}, newTestSource())

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Test"}},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 1 {
		t.Errorf("EntitiesCreated = %d, want 1", result.EntitiesCreated)
	}

	// Check that store is empty (dry run)
	n, _ := st.CountEntities(ctx(), store.EntityQuery{})
	if n != 0 {
		t.Errorf("Expected empty store in dry run, found %d entities", n)
	}
}

func TestImportEntities(t *testing.T) {
	st := newTestStore()
	meta := testMetamodel()

	imp := New(st, meta, Options{}, newTestSource())

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "First requirement", "status": "draft"}},
			{ID: "REQ-002", Type: "requirement", Properties: map[string]interface{}{"title": "Second requirement", "status": "accepted"}},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 2 {
		t.Errorf("EntitiesCreated = %d, want 2", result.EntitiesCreated)
	}

	n, _ := st.CountEntities(ctx(), store.EntityQuery{})
	if n != 2 {
		t.Errorf("Store entities = %d, want 2", n)
	}

	e, err := st.GetEntity(ctx(), "REQ-001")
	if err != nil {
		t.Error("REQ-001 not found in store")
	} else if e.Title() != "First requirement" {
		t.Errorf("REQ-001 title = %q, want %q", e.Title(), "First requirement")
	}
}

func TestImportWithRelations(t *testing.T) {
	st := newTestStore()
	meta := testMetamodel()

	imp := New(st, meta, Options{}, newTestSource())

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Requirement"}},
			{ID: "DEC-001", Type: "decision", Properties: map[string]interface{}{"title": "Decision"}},
		},
		Relations: []RelationData{
			{From: "DEC-001", Relation: "addresses", To: "REQ-001"},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 2 {
		t.Errorf("EntitiesCreated = %d, want 2", result.EntitiesCreated)
	}
	if result.RelationsCreated != 1 {
		t.Errorf("RelationsCreated = %d, want 1", result.RelationsCreated)
	}

	if _, err := st.GetRelation(ctx(), "DEC-001", "addresses", "REQ-001"); err != nil {
		t.Error("Relation DEC-001 --addresses--> REQ-001 not found in store")
	}
}

func TestImportValidationErrors(t *testing.T) {
	meta := testMetamodel()

	tests := []struct {
		name    string
		data    *ImportData
		wantErr string
	}{
		{
			name: "missing id",
			data: &ImportData{
				Entities: []EntityData{{Type: "requirement", Properties: map[string]interface{}{"title": "Test"}}},
			},
			wantErr: "missing required field: id",
		},
		{
			name: "missing type",
			data: &ImportData{
				Entities: []EntityData{{ID: "REQ-001", Properties: map[string]interface{}{"title": "Test"}}},
			},
			wantErr: "missing required field: type",
		},
		{
			name: "unknown type",
			data: &ImportData{
				Entities: []EntityData{{ID: "FOO-001", Type: "unknown", Properties: map[string]interface{}{"title": "Test"}}},
			},
			wantErr: "unknown entity type",
		},
		{
			name: "missing required property",
			data: &ImportData{
				Entities: []EntityData{{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{}}},
			},
			wantErr: "This field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestStore()
			imp := New(st, meta, Options{}, newTestSource())
			_, err := imp.Import(tt.data)
			if err == nil {
				t.Error("Expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestImportUpdate(t *testing.T) {
	st := newTestStore()
	meta := testMetamodel()
	src := newTestSource()

	// First import
	imp := New(st, meta, Options{}, src)
	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Original"}},
		},
	}
	_, err := imp.Import(data)
	if err != nil {
		t.Fatalf("First import error: %v", err)
	}

	// Second import without update - should fail
	imp2 := New(st, meta, Options{}, src)
	data2 := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Updated"}},
		},
	}
	_, err = imp2.Import(data2)
	if err == nil {
		t.Error("Expected error for duplicate without --update")
	}

	// Third import with update - should succeed
	imp3 := New(st, meta, Options{Update: true}, src)
	result, err := imp3.Import(data2)
	if err != nil {
		t.Fatalf("Update import error: %v", err)
	}
	if result.EntitiesUpdated != 1 {
		t.Errorf("EntitiesUpdated = %d, want 1", result.EntitiesUpdated)
	}

	// Check title was updated
	e, _ := st.GetEntity(ctx(), "REQ-001")
	if e.Title() != "Updated" {
		t.Errorf("Title = %q, want %q", e.Title(), "Updated")
	}
}

func TestImportSkipErrors(t *testing.T) {
	st := newTestStore()
	meta := testMetamodel()

	imp := New(st, meta, Options{SkipErrors: true}, newTestSource())

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "Valid"}},
			{ID: "BAD-001", Type: "unknown", Properties: map[string]interface{}{"title": "Invalid type"}},
			{ID: "REQ-002", Type: "requirement", Properties: map[string]interface{}{"title": "Also valid"}},
		},
	}

	result, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.EntitiesCreated != 2 {
		t.Errorf("EntitiesCreated = %d, want 2", result.EntitiesCreated)
	}
	if result.EntitiesSkipped != 1 {
		t.Errorf("EntitiesSkipped = %d, want 1", result.EntitiesSkipped)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Errors = %d, want 1", len(result.Errors))
	}
}

func TestImportDefaultStatus(t *testing.T) {
	st := newTestStore()
	meta := testMetamodel()

	imp := New(st, meta, Options{}, newTestSource())

	data := &ImportData{
		Entities: []EntityData{
			{ID: "REQ-001", Type: "requirement", Properties: map[string]interface{}{"title": "No status"}},
		},
	}

	_, err := imp.Import(data)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	e, _ := st.GetEntity(ctx(), "REQ-001")
	status := e.GetString("status")
	if status != "draft" {
		t.Errorf("Status = %q, want %q", status, "draft")
	}
}
