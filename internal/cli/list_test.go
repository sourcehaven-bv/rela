package cli

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func setupListTestEnv() {
	g = graph.New()
	meta = nil // Will be set by individual tests
	ws = nil   // Will be set by individual tests after meta is set
	out = output.New(output.FormatTable)
	projectCtx = &project.Context{
		Root:          "/tmp/test-project",
		EntitiesDir:   "/tmp/test-project/entities",
		RelationsDir:  "/tmp/test-project/relations",
		CacheDir:      "/tmp/test-project/.rela",
		CachePath:     "/tmp/test-project/.rela/cache.json",
		MetamodelPath: "/tmp/test-project/metamodel.yaml",
	}
}

// setupWorkspaceFromMeta creates a workspace backed by a MemFS so that
// resolveEntityType (which now delegates to ws) works in tests.
func setupWorkspaceFromMeta(t *testing.T, m *metamodel.Metamodel) {
	t.Helper()
	fs := storage.NewMemFS()
	ctx := &project.Context{
		Root: "/tmp/test-project", MetamodelPath: "/tmp/test-project/metamodel.yaml",
		CacheDir: "/tmp/test-project/.rela", CachePath: "/tmp/test-project/.rela/cache.json",
		EntitiesDir: "/tmp/test-project/entities", RelationsDir: "/tmp/test-project/relations",
	}
	_ = fs.MkdirAll(ctx.EntitiesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	repo := repository.New(fs, ctx)
	ws = workspace.NewWithGraph(repo, m, g)
}

func TestResolveEntityTypeWithAlias(t *testing.T) {
	setupListTestEnv()

	// Use shared alias metamodel
	var err error
	meta, err = metamodel.Parse([]byte(testutil.AliasMetamodelYAML()))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}
	setupWorkspaceFromMeta(t, meta)

	tests := []struct {
		name      string
		input     string
		wantType  string
		wantError bool
	}{
		{
			name:     "canonical name",
			input:    "requirement",
			wantType: "requirement",
		},
		{
			name:     "alias req",
			input:    "req",
			wantType: "requirement",
		},
		{
			name:     "plural form",
			input:    "requirements",
			wantType: "requirement",
		},
		{
			name:     "alias ctrl",
			input:    "ctrl",
			wantType: "control",
		},
		{
			name:     "plural controls",
			input:    "controls",
			wantType: "control",
		},
		{
			name:      "unknown type",
			input:     "unknown",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, _, err := resolveEntityType(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("resolveEntityType(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("resolveEntityType(%q) unexpected error: %v", tt.input, err)
				return
			}
			if resolved != tt.wantType {
				t.Errorf("resolveEntityType(%q) = %q, want %q", tt.input, resolved, tt.wantType)
			}
		})
	}
}

func TestListCommandWithAliases(t *testing.T) {
	setupListTestEnv()

	// Use shared alias metamodel (has requirement with 'req' alias)
	var err error
	meta, err = metamodel.Parse([]byte(testutil.AliasMetamodelYAML()))
	if err != nil {
		t.Fatalf("failed to parse metamodel: %v", err)
	}
	ws = workspace.NewForTest(g, meta)

	// Add some test entities to the graph
	g.AddNode(testutil.Entity("requirement").
		ID("REQ-001").
		With("title", "Test requirement").
		With("status", "draft").
		Build())

	// Test using alias directly
	resolved, def, err := resolveEntityType("req")
	if err != nil {
		t.Fatalf("resolveEntityType(\"req\") failed: %v", err)
	}
	if resolved != "requirement" {
		t.Errorf("resolveEntityType(\"req\") = %q, want %q", resolved, "requirement")
	}
	if def == nil {
		t.Error("resolveEntityType(\"req\") returned nil definition")
	}

	// Verify we can get entities by the resolved type
	entities := g.NodesByType(resolved)
	if len(entities) != 1 {
		t.Errorf("NodesByType(%q) = %d entities, want 1", resolved, len(entities))
	}
}

// TestListTypeParsingEdgeCases tests edge cases for entity type resolution
// including entity types and aliases that end in 's' (like "bus", "autobus")
func TestListTypeParsingEdgeCases(t *testing.T) {
	setupListTestEnv()

	// Build metamodel with entity type ending in 's' (edge case)
	meta = testutil.NewMetamodel().
		DefineEntity("requirement").
		Label("Requirement").
		IDPrefix("REQ-").
		Aliases("req").
		Prop("title", metamodel.PropertyTypeString, true).
		Prop("status", "status", true).
		End().
		DefineEntity("bus").
		Label("Bus").
		IDPrefix("BUS-").
		Aliases("autobus").
		Prop("title", metamodel.PropertyTypeString, true).
		End().
		WithCustomTypeDefault("status", []string{"draft", "accepted"}, "draft").
		Build()
	setupWorkspaceFromMeta(t, meta)

	// Test cases that the list command should handle correctly
	// The fix ensures that alias resolution happens BEFORE plural stripping
	tests := []struct {
		name      string
		input     string
		wantType  string
		wantError bool
	}{
		{
			name:     "canonical name requirement",
			input:    "requirement",
			wantType: "requirement",
		},
		{
			name:     "alias req",
			input:    "req",
			wantType: "requirement",
		},
		{
			name:     "plural requirements",
			input:    "requirements",
			wantType: "requirement",
		},
		// Edge case: entity type that ends in 's' (bus)
		// This was the bug - "bus" was being incorrectly stripped to "bu"
		{
			name:     "canonical name bus (ends in s)",
			input:    "bus",
			wantType: "bus",
		},
		// Edge case: alias that ends in 's' (autobus)
		// This was also a bug - "autobus" was being stripped to "autobu"
		{
			name:     "alias autobus (ends in s)",
			input:    "autobus",
			wantType: "bus",
		},
		// Plural of bus should still work
		{
			name:     "plural buses",
			input:    "buses",
			wantType: "bus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call resolveEntityType directly - this is what list.go now does
			resolved, _, err := resolveEntityType(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("resolveEntityType(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("resolveEntityType(%q) unexpected error: %v", tt.input, err)
				return
			}
			if resolved != tt.wantType {
				t.Errorf("resolveEntityType(%q) = %q, want %q", tt.input, resolved, tt.wantType)
			}
		})
	}
}

func TestListAllEntities(t *testing.T) {
	setupListTestEnv()

	meta = metamodel.DefaultMetamodel()
	ws = workspace.NewForTest(g, meta)

	// Add entities of different types
	g.AddNode(testutil.Entity("requirement").ID("REQ-001").With("title", "Test Requirement").Build())
	g.AddNode(testutil.Entity("decision").ID("DEC-001").With("title", "Test Decision").Build())

	// List all entities (no type filter)
	entities := g.AllNodes()
	if len(entities) != 2 {
		t.Errorf("AllNodes() = %d entities, want 2", len(entities))
	}
}

func TestListEmptyGraph(t *testing.T) {
	setupListTestEnv()
	meta = metamodel.DefaultMetamodel()
	ws = workspace.NewForTest(g, meta)

	// Empty graph
	entities := g.AllNodes()
	if len(entities) != 0 {
		t.Errorf("AllNodes() = %d entities, want 0", len(entities))
	}
}

func TestListByType(t *testing.T) {
	setupListTestEnv()
	meta = metamodel.DefaultMetamodel()
	ws = workspace.NewForTest(g, meta)

	// Add entities
	g.AddNode(testutil.Entity("requirement").ID("REQ-001").With("title", "Req 1").Build())
	g.AddNode(testutil.Entity("requirement").ID("REQ-002").With("title", "Req 2").Build())
	g.AddNode(testutil.Entity("decision").ID("DEC-001").With("title", "Dec 1").Build())

	// List only requirements
	entities := g.NodesByType("requirement")
	if len(entities) != 2 {
		t.Errorf("NodesByType(requirement) = %d entities, want 2", len(entities))
	}

	// Verify they are requirements
	for _, e := range entities {
		if e.Type != "requirement" {
			t.Errorf("expected type 'requirement', got %s", e.Type)
		}
	}
}

func TestListCommandWithUnknownType(t *testing.T) {
	setupListTestEnv()
	meta = metamodel.DefaultMetamodel()
	ws = workspace.NewForTest(g, meta)

	_, _, err := resolveEntityType("nonexistent")
	if err == nil {
		t.Error("expected error for unknown entity type")
	}

	if !strings.Contains(err.Error(), "unknown entity type") {
		t.Errorf("expected 'unknown entity type' in error, got: %v", err)
	}
}
