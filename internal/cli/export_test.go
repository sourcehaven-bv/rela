package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// setupExportGraph builds a small graph and returns the services and
// metamodel for export tests to share.
func setupExportGraph(t *testing.T) (*cliServices, *metamodel.Metamodel) {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"control": {
				Label:    "Control",
				IDPrefix: "CTRL-",
			},
			"risk": {
				Label:    "Risk",
				IDPrefix: "RISK-",
			},
			"evidence": {
				Label:    "Evidence",
				IDPrefix: "EV-",
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"mitigates": {
				From: []string{"control"},
				To:   []string{"risk"},
			},
			"evidencedBy": {
				From: []string{"control"},
				To:   []string{"evidence"},
			},
		},
	}

	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "control").
		ID("CTRL-001").
		With("title", "Access Control Policy").
		With("status", "implemented").
		With("iso27001", "A.5.15"))

	seeder.addEntity(testutil.EntityFor(meta, "control").
		ID("CTRL-002").
		With("title", "Password Policy").
		With("status", "draft").
		With("iso27001", "A.9.4.3"))

	seeder.addEntity(testutil.EntityFor(meta, "risk").
		ID("RISK-001").
		With("title", "Unauthorized Access").
		With("severity", "high"))

	seeder.addEntity(testutil.EntityFor(meta, "evidence").
		ID("EV-001").
		With("title", "Access Control Audit Report").
		With("valid_until", "2025-12-31"))

	seeder.addRelation("CTRL-001", "mitigates", "RISK-001")
	seeder.addRelation("CTRL-002", "mitigates", "RISK-001")
	seeder.addRelation("CTRL-001", "evidencedBy", "EV-001")

	return seeder.build(t), meta
}

// captureStdout runs fn while redirecting os.Stdout into a buffer.
// Returns the captured output.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), err
}

func TestExportEntitiesJSON(t *testing.T) {
	svc, _ := setupExportGraph(t)
	cmd := &ExportCmd{Format: "json"}

	output, err := captureStdout(t, func() error {
		return cmd.exportEntities(context.Background(), svc, "control")
	})
	if err != nil {
		t.Fatalf("exportEntities failed: %v", err)
	}

	var entities []ExportEntity
	if err := json.Unmarshal([]byte(output), &entities); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput was: %s", err, output)
	}

	if len(entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(entities))
	}

	if entities[0].ID != "CTRL-001" {
		t.Errorf("Expected first entity to be CTRL-001, got %s", entities[0].ID)
	}
	if entities[0].Type != "control" {
		t.Errorf("Expected type 'control', got %s", entities[0].Type)
	}
	if entities[0].Properties["title"] != "Access Control Policy" {
		t.Errorf("Unexpected title: %v", entities[0].Properties["title"])
	}
}

func TestExportEntitiesWithRelations(t *testing.T) {
	svc, _ := setupExportGraph(t)
	cmd := &ExportCmd{Format: "json", WithRelations: true}

	output, err := captureStdout(t, func() error {
		return cmd.exportEntities(context.Background(), svc, "control")
	})
	if err != nil {
		t.Fatalf("exportEntities failed: %v", err)
	}

	var entities []ExportEntity
	if err := json.Unmarshal([]byte(output), &entities); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	var ctrl1 *ExportEntity
	for i := range entities {
		if entities[i].ID == "CTRL-001" {
			ctrl1 = &entities[i]
			break
		}
	}

	if ctrl1 == nil {
		t.Fatal("CTRL-001 not found in output")
	}

	if ctrl1.Relations == nil {
		t.Fatal("Expected relations for CTRL-001")
	}

	if ctrl1.Relations.Outgoing == nil {
		t.Fatal("Expected outgoing relations")
	}

	mitigates := ctrl1.Relations.Outgoing["mitigates"]
	if len(mitigates) != 1 {
		t.Errorf("Expected 1 mitigates relation, got %d", len(mitigates))
	}
	if mitigates[0].ID != "RISK-001" {
		t.Errorf("Expected target RISK-001, got %s", mitigates[0].ID)
	}

	evidencedBy := ctrl1.Relations.Outgoing["evidencedBy"]
	if len(evidencedBy) != 1 {
		t.Errorf("Expected 1 evidencedBy relation, got %d", len(evidencedBy))
	}
}

func TestExportEntitiesCSV(t *testing.T) {
	svc, _ := setupExportGraph(t)
	cmd := &ExportCmd{Format: "csv"}

	entities := fixtureEntities(t, svc, "control")
	exportData := make([]ExportEntity, 0, len(entities))
	for _, e := range entities {
		exportData = append(exportData, entityToExport(e))
	}

	output, err := captureStdout(t, func() error {
		return cmd.writeCSV(exportData, entities)
	})
	if err != nil {
		t.Fatalf("writeCSV failed: %v", err)
	}

	reader := csv.NewReader(strings.NewReader(output))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(records) < 1 {
		t.Fatal("Expected at least header row")
	}

	header := records[0]
	if header[0] != "id" || header[1] != "type" {
		t.Errorf("Unexpected header: %v", header)
	}

	hasTitle := false
	hasStatus := false
	for _, h := range header {
		if h == "title" {
			hasTitle = true
		}
		if h == "status" {
			hasStatus = true
		}
	}
	if !hasTitle || !hasStatus {
		t.Errorf("Missing expected columns. Header: %v", header)
	}
}

func TestExportEntitiesYAML(t *testing.T) {
	svc, _ := setupExportGraph(t)
	cmd := &ExportCmd{Format: "yaml"}

	output, err := captureStdout(t, func() error {
		return cmd.exportEntities(context.Background(), svc, "control")
	})
	if err != nil {
		t.Fatalf("exportEntities failed: %v", err)
	}

	var entities []ExportEntity
	if err := yaml.Unmarshal([]byte(output), &entities); err != nil {
		t.Fatalf("Failed to parse YAML output: %v\nOutput was: %s", err, output)
	}

	if len(entities) != 2 {
		t.Errorf("Expected 2 entities, got %d", len(entities))
	}
}

func TestExportAllData(t *testing.T) {
	svc, _ := setupExportGraph(t)
	cmd := &ExportCmd{Format: "json", All: true}

	output, err := captureStdout(t, func() error {
		return cmd.exportAllData(context.Background(), svc)
	})
	if err != nil {
		t.Fatalf("exportAllData failed: %v", err)
	}

	var fullExport FullExport
	if err := json.Unmarshal([]byte(output), &fullExport); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if len(fullExport.Entities) != 4 {
		t.Errorf("Expected 4 entities, got %d", len(fullExport.Entities))
	}

	if len(fullExport.Relations) != 3 {
		t.Errorf("Expected 3 relations, got %d", len(fullExport.Relations))
	}
}

func TestExportEmptyResult(t *testing.T) {
	svc, meta := setupExportGraph(t)
	cmd := &ExportCmd{Format: "json"}

	// Export a type that exists in metamodel but has no entities.
	meta.Entities["procedure"] = metamodel.EntityDef{
		Label:    "Procedure",
		IDPrefix: "PROC-",
	}

	output, err := captureStdout(t, func() error {
		return cmd.exportEntities(context.Background(), svc, "procedure")
	})
	if err != nil {
		t.Fatalf("exportEntities failed: %v", err)
	}

	trimmed := strings.TrimSpace(output)
	if trimmed != "[]" {
		t.Errorf("Expected empty array '[]', got: %s", trimmed)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"simple string", "simple string"},
		{nil, ""},
		{123, "123"},
		{true, "true"},
		{[]string{"a", "b"}, `["a","b"]`},
	}

	for _, tt := range tests {
		result := formatValue(tt.input)
		if result != tt.expected {
			t.Errorf("formatValue(%v) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestFormatRelationsMap(t *testing.T) {
	result := formatRelationsMap(nil)
	if result != "" {
		t.Errorf("Expected empty string for nil map, got: %s", result)
	}

	result = formatRelationsMap(map[string][]RelationTarget{})
	if result != "" {
		t.Errorf("Expected empty string for empty map, got: %s", result)
	}

	m := map[string][]RelationTarget{
		"mitigates": {{ID: "RISK-001"}},
	}
	result = formatRelationsMap(m)
	if result != "mitigates:RISK-001" {
		t.Errorf("Unexpected result: %s", result)
	}

	m = map[string][]RelationTarget{
		"mitigates": {{ID: "RISK-001"}, {ID: "RISK-002"}},
	}
	result = formatRelationsMap(m)
	if result != "mitigates:RISK-001,RISK-002" {
		t.Errorf("Unexpected result: %s", result)
	}

	m = map[string][]RelationTarget{
		"mitigates":   {{ID: "RISK-001"}},
		"evidencedBy": {{ID: "EV-001"}},
	}
	result = formatRelationsMap(m)
	if result != "evidencedBy:EV-001;mitigates:RISK-001" {
		t.Errorf("Unexpected result: %s", result)
	}
}

func TestCollectPropertyKeys(t *testing.T) {
	entities := []*entity.Entity{
		{Properties: map[string]interface{}{"title": "A", "foo": "bar"}},
		{Properties: map[string]interface{}{"title": "B", "status": "draft", "baz": "qux"}},
	}

	keys := collectPropertyKeys(entities)

	if len(keys) < 2 {
		t.Fatalf("Expected at least 2 keys, got %d", len(keys))
	}
	if keys[0] != "title" {
		t.Errorf("Expected 'title' first, got %s", keys[0])
	}
	if keys[1] != "status" {
		t.Errorf("Expected 'status' second, got %s", keys[1])
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, expected := range []string{"title", "status", "foo", "baz"} {
		if !keySet[expected] {
			t.Errorf("Missing expected key: %s", expected)
		}
	}
}
