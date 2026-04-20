package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// show_test.go focuses on CLI-specific concerns:
//   - argument parsing (entity-not-found, exact-arg validation)
//   - output rendering for both table and JSON formats
//
// Entity/relation read semantics are covered by the store conformance
// suite (internal/store/storetest). Representative formatting tests
// here are enough to catch regressions in the CLI glue.

// setupShowTest wires globals that the show command reads.
func setupShowTest(t *testing.T) (buf *bytes.Buffer, seeder *storeSeeder, cleanup func()) {
	t.Helper()

	oldMeta := meta
	oldOut := out
	oldWs := ws

	meta = metamodel.DefaultMetamodel()
	seeder = newStoreSeeder(meta)

	buf = new(bytes.Buffer)
	out = output.NewWithWriter(buf, output.FormatTable)

	cleanup = func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}

	return buf, seeder, cleanup
}

// TestShowEntityRendersRelations covers the full CLI render path: entity
// details plus incoming and outgoing relations in the default (table)
// output. One representative case is enough — the store's own tests
// cover the query side.
func TestShowEntityRendersRelations(t *testing.T) {
	buf, seeder, cleanup := setupShowTest(t)
	defer cleanup()

	seeder.addEntity(testutil.EntityFor(meta, "requirement").ID("REQ-001").
		With("status", "draft"))
	seeder.addEntity(testutil.EntityFor(meta, "decision").ID("DEC-001"))
	seeder.addEntity(testutil.EntityFor(meta, "solution").ID("SOL-001"))
	seeder.addRelation("DEC-001", "addresses", "REQ-001")
	seeder.addRelation("REQ-001", "implements", "SOL-001")
	applySeeder(seeder)

	if err := showCmd.RunE(showCmd, []string{"REQ-001"}); err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	result := buf.String()
	for _, want := range []string{"REQ-001", "draft", "DEC-001", "addresses", "SOL-001", "implements"} {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in output, got: %s", want, result)
		}
	}
}

// TestShowEntityNotFound covers the error path for a missing ID.
func TestShowEntityNotFound(t *testing.T) {
	_, seeder, cleanup := setupShowTest(t)
	defer cleanup()
	applySeeder(seeder)

	err := showCmd.RunE(showCmd, []string{"NONEXISTENT-001"})
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}

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

// TestShowEntityJSON exercises JSON output shape, which is a CLI
// concern distinct from the table formatter.
func TestShowEntityJSON(t *testing.T) {
	oldMeta := meta
	oldOut := out
	oldWs := ws
	defer func() {
		meta = oldMeta
		out = oldOut
		ws = oldWs
	}()

	meta = metamodel.DefaultMetamodel()
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "requirement").ID("REQ-001"))

	var buf bytes.Buffer
	out = output.NewWithWriter(&buf, output.FormatJSON)
	applySeeder(seeder)

	if err := showCmd.RunE(showCmd, []string{"REQ-001"}); err != nil {
		t.Fatalf("show command failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	entityData, ok := result["entity"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'entity' key in JSON output")
	}

	if entityData["id"] != "REQ-001" {
		t.Errorf("expected id='REQ-001', got: %v", entityData["id"])
	}
	if entityData["type"] != "requirement" {
		t.Errorf("expected type='requirement', got: %v", entityData["type"])
	}

	props, ok := entityData["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties to be a map")
	}
	if props["title"] == "" {
		t.Errorf("expected title to be non-empty, got: %v", props["title"])
	}
}

// TestEntityNotFoundError covers the custom error type's Error()
// message format — independent of CLI globals.
func TestEntityNotFoundError(t *testing.T) {
	err := &entityNotFoundError{ID: "TEST-123"}

	expected := "entity not found: TEST-123"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// TestClassifyReadError covers the four branches of the error
// classifier so a future change to one branch can't silently regress.
func TestClassifyReadError(t *testing.T) {
	notFound := classifyReadError("REQ-001", store.ErrNotFound)
	var nfe *entityNotFoundError
	if !errors.As(notFound, &nfe) {
		t.Errorf("ErrNotFound should classify as entityNotFoundError, got %T", notFound)
	}

	noMatch := classifyReadError("REQ-002",
		fmt.Errorf("wrap: %w", encryption.ErrNoMatchingKey))
	if !strings.Contains(noMatch.Error(), "not authorized") {
		t.Errorf("no-matching-key error missing 'not authorized': %v", noMatch)
	}

	noKey := classifyReadError("REQ-003",
		fmt.Errorf("wrap: %w", encryption.ErrNoPrivateKey))
	if !strings.Contains(noKey.Error(), "no identity loaded") {
		t.Errorf("no-private-key error missing 'no identity loaded': %v", noKey)
	}

	corrupt := classifyReadError("REQ-004",
		fmt.Errorf("wrap: %w", encryption.ErrCorrupted))
	if !strings.Contains(corrupt.Error(), "corrupted") {
		t.Errorf("corrupted error missing 'corrupted': %v", corrupt)
	}

	other := errors.New("some other error")
	passthrough := classifyReadError("REQ-005", other)
	if !errors.Is(passthrough, other) {
		t.Errorf("unknown errors should pass through unchanged, got %v", passthrough)
	}
}

// TestShowCommandRequiresExactlyOneArg exercises the cobra Args
// validator — a CLI plumbing concern.
func TestShowCommandRequiresExactlyOneArg(t *testing.T) {
	_, seeder, cleanup := setupShowTest(t)
	defer cleanup()
	applySeeder(seeder)

	err := showCmd.Args(showCmd, []string{})
	if err == nil {
		t.Error("expected error when no arguments provided")
	}

	err = showCmd.Args(showCmd, []string{"REQ-001", "REQ-002"})
	if err == nil {
		t.Error("expected error when multiple arguments provided")
	}

	err = showCmd.Args(showCmd, []string{"REQ-001"})
	if err != nil {
		t.Errorf("unexpected error with one argument: %v", err)
	}
}
