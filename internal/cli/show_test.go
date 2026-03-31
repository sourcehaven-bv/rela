package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// setupShowTest sets up the test environment for show command tests
func setupShowTest(t *testing.T) (buf *bytes.Buffer, cleanup func()) {
	t.Helper()

	oldMeta := meta
	oldG := g
	oldOut := out
	oldWs := ws

	meta = metamodel.DefaultMetamodel()
	g = graph.New()
	ws = workspace.NewForTest(g, meta)

	buf = new(bytes.Buffer)
	out = output.NewWithWriter(buf, output.FormatTable)

	cleanup = func() {
		meta = oldMeta
		g = oldG
		out = oldOut
		ws = oldWs
	}

	return buf, cleanup
}

func TestShowEntity(t *testing.T) {
	buf, cleanup := setupShowTest(t)
	defer cleanup()

	// Add a test entity
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		With("status", "draft").
		With("priority", "high").
		Build())

	err := showCmd.RunE(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	result := buf.String()

	// Check for entity ID and properties
	if !strings.Contains(result, "REQ-001") {
		t.Errorf("expected 'REQ-001' in output, got: %s", result)
	}
	if !strings.Contains(result, "draft") {
		t.Errorf("expected 'draft' status in output, got: %s", result)
	}
}

func TestShowEntityWithIncomingRelations(t *testing.T) {
	buf, cleanup := setupShowTest(t)
	defer cleanup()

	// Add entities
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		Build())

	g.AddNode(testutil.EntityFor(meta, "decision").
		ID("DEC-001").
		Build())

	// Add relation: DEC-001 addresses REQ-001
	g.AddEdge(testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build())

	err := showCmd.RunE(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	result := buf.String()

	// Should show the incoming relation from DEC-001
	if !strings.Contains(result, "DEC-001") {
		t.Errorf("expected incoming relation from 'DEC-001' in output, got: %s", result)
	}
	if !strings.Contains(result, "addresses") {
		t.Errorf("expected 'addresses' relation in output, got: %s", result)
	}
}

func TestShowEntityWithOutgoingRelations(t *testing.T) {
	buf, cleanup := setupShowTest(t)
	defer cleanup()

	// Add entities
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		Build())

	g.AddNode(testutil.EntityFor(meta, "decision").
		ID("DEC-001").
		Build())

	// Add relation: DEC-001 addresses REQ-001
	g.AddEdge(testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build())

	err := showCmd.RunE(showCmd, []string{"DEC-001"})
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	result := buf.String()

	// Should show the outgoing relation to REQ-001
	if !strings.Contains(result, "REQ-001") {
		t.Errorf("expected outgoing relation to 'REQ-001' in output, got: %s", result)
	}
	if !strings.Contains(result, "addresses") {
		t.Errorf("expected 'addresses' relation in output, got: %s", result)
	}
}

func TestShowEntityNotFound(t *testing.T) {
	_, cleanup := setupShowTest(t)
	defer cleanup()

	err := showCmd.RunE(showCmd, []string{"NONEXISTENT-001"})
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}

	// Should be entityNotFoundError
	var notFoundErr *entityNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("expected entityNotFoundError, got: %T", err)
	}

	if !strings.Contains(err.Error(), "entity not found") {
		t.Errorf("expected 'entity not found' in error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "NONEXISTENT-001") {
		t.Errorf("expected 'NONEXISTENT-001' in error message, got: %v", err)
	}
}

func TestShowEntityJSON(t *testing.T) {
	oldMeta := meta
	oldG := g
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		g = oldG
		out = oldOut
		ws = oldWs
	}()

	meta = metamodel.DefaultMetamodel()
	g = graph.New()
	ws = workspace.NewForTest(g, meta)

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)

	// Add a test entity
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		Build())

	err := showCmd.RunE(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// The JSON output wraps the entity in an "entity" key
	entityData, ok := result["entity"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'entity' key in JSON output")
	}

	// Check for expected fields
	if entityData["id"] != "REQ-001" {
		t.Errorf("expected id='REQ-001', got: %v", entityData["id"])
	}
	if entityData["type"] != "requirement" {
		t.Errorf("expected type='requirement', got: %v", entityData["type"])
	}

	// Check properties - EntityFor auto-fills title and status
	props, ok := entityData["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties to be a map")
	}
	if props["title"] == "" {
		t.Errorf("expected title to be non-empty, got: %v", props["title"])
	}
}

func TestShowEntityWithMultipleRelations(t *testing.T) {
	buf, cleanup := setupShowTest(t)
	defer cleanup()

	// Add entities
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		Build())

	g.AddNode(testutil.EntityFor(meta, "decision").
		ID("DEC-001").
		Build())

	g.AddNode(testutil.EntityFor(meta, "decision").
		ID("DEC-002").
		Build())

	g.AddNode(testutil.EntityFor(meta, "solution").
		ID("SOL-001").
		Build())

	// Add relations
	g.AddEdge(testutil.NewRelation("DEC-001", "addresses", "REQ-001").Build())
	g.AddEdge(testutil.NewRelation("DEC-002", "addresses", "REQ-001").Build())
	g.AddEdge(testutil.NewRelation("REQ-001", "implements", "SOL-001").Build())

	err := showCmd.RunE(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	result := buf.String()

	// Should show both incoming relations
	if !strings.Contains(result, "DEC-001") {
		t.Errorf("expected 'DEC-001' in output, got: %s", result)
	}
	if !strings.Contains(result, "DEC-002") {
		t.Errorf("expected 'DEC-002' in output, got: %s", result)
	}

	// Should show outgoing relation
	if !strings.Contains(result, "SOL-001") {
		t.Errorf("expected 'SOL-001' in output, got: %s", result)
	}
}

func TestShowEntityWithNoRelations(t *testing.T) {
	buf, cleanup := setupShowTest(t)
	defer cleanup()

	// Add a standalone entity with no relations
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		Build())

	err := showCmd.RunE(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	result := buf.String()

	// Should still show the entity
	if !strings.Contains(result, "REQ-001") {
		t.Errorf("expected 'REQ-001' in output, got: %s", result)
	}
}

func TestEntityNotFoundError(t *testing.T) {
	err := &entityNotFoundError{ID: "TEST-123"}

	expected := "entity not found: TEST-123"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestShowCommandRequiresExactlyOneArg(t *testing.T) {
	_, cleanup := setupShowTest(t)
	defer cleanup()

	// Test with no arguments
	err := showCmd.Args(showCmd, []string{})
	if err == nil {
		t.Error("expected error when no arguments provided")
	}

	// Test with multiple arguments
	err = showCmd.Args(showCmd, []string{"REQ-001", "REQ-002"})
	if err == nil {
		t.Error("expected error when multiple arguments provided")
	}

	// Test with exactly one argument (should succeed)
	err = showCmd.Args(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Errorf("unexpected error with one argument: %v", err)
	}
}

func TestShowEntityWithContent(t *testing.T) {
	buf, cleanup := setupShowTest(t)
	defer cleanup()

	// Add entity with content
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		WithContent("This is the detailed description of the requirement.\n\nIt can span multiple lines.").
		Build())

	err := showCmd.RunE(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	result := buf.String()

	// Should include the content
	if !strings.Contains(result, "detailed description") {
		t.Errorf("expected content in output, got: %s", result)
	}
}
