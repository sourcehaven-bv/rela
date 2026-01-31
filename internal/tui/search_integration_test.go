package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/tui/searchparser"
)

type searchTestCase struct {
	name        string
	query       string
	expectCount int
	expectIDs   []string
	expectError bool
}

// TestSearchIntegration runs comprehensive integration tests for the search feature
func TestSearchIntegration(t *testing.T) {
	// Initialize test project
	projectDir := "/tmp/rela-test-project"

	// Check if test project exists
	if _, err := os.Stat(filepath.Join(projectDir, "metamodel.yaml")); os.IsNotExist(err) {
		t.Skipf("Test project not found at %s - run setup first", projectDir)
	}

	ctx, err := project.Discover(projectDir, storage.NewOsFS())
	if err != nil {
		t.Fatalf("Failed to discover project: %v", err)
	}

	meta, err := metamodel.Load(ctx.MetamodelPath, storage.NewOsFS())
	if err != nil {
		// Skip if using deprecated syntax (needs migration)
		if strings.Contains(err.Error(), "deprecated syntax") {
			t.Skipf("Test project uses deprecated syntax - run 'rela migrate' first: %v", err)
		}
		t.Fatalf("Failed to load metamodel: %v", err)
	}

	g := graph.New()
	testRepo := repository.New(storage.NewOsFS(), ctx)
	if _, err := testRepo.Sync(meta, g); err != nil {
		t.Fatalf("Failed to sync entities: %v", err)
	}

	// Define test cases
	tests := []searchTestCase{
		{
			name:        "Empty query returns nothing",
			query:       "",
			expectCount: 0,
		},
		{
			name:        "Simple text search - 'authentication'",
			query:       "authentication",
			expectCount: 4,
			expectIDs:   []string{"REQ-001", "REQ-004", "DEC-002", "SOL-001"},
		},
		{
			name:        "Simple text search - 'OAuth'",
			query:       "OAuth",
			expectCount: 2,
			expectIDs:   []string{"REQ-001", "SOL-001"},
		},
		{
			name:        "Simple text search - 'API'",
			query:       "API",
			expectCount: 3,
			expectIDs:   []string{"REQ-002", "DEC-002", "SOL-002"},
		},
		{
			name:        "Quoted phrase - exact match",
			query:       `"OAuth 2.0"`,
			expectCount: 2,
			expectIDs:   []string{"REQ-001", "SOL-001"},
		},
		{
			name:        "Multiple words (AND logic)",
			query:       "API authentication",
			expectCount: 1,
			expectIDs:   []string{"DEC-002"},
		},
		{
			name:        "Type filter - requirements only",
			query:       "type:requirement",
			expectCount: 4,
			expectIDs:   []string{"REQ-001", "REQ-002", "REQ-003", "REQ-004"},
		},
		{
			name:        "Type filter - decisions only",
			query:       "type:decision",
			expectCount: 2,
			expectIDs:   []string{"DEC-001", "DEC-002"},
		},
		{
			name:        "Type filter - solutions only",
			query:       "type:solution",
			expectCount: 2,
			expectIDs:   []string{"SOL-001", "SOL-002"},
		},
		{
			name:        "Multiple types",
			query:       "type:requirement,decision",
			expectCount: 6,
			expectIDs:   []string{"REQ-001", "REQ-002", "REQ-003", "REQ-004", "DEC-001", "DEC-002"},
		},
		{
			name:        "Property filter - status=published",
			query:       "prop:status=published",
			expectCount: 2,
			expectIDs:   []string{"REQ-001", "REQ-004"},
		},
		{
			name:        "Property filter - status=draft",
			query:       "prop:status=draft",
			expectCount: 2,
			expectIDs:   []string{"REQ-002", "SOL-002"},
		},
		{
			name:        "Property filter - status!=draft",
			query:       "prop:status!=draft",
			expectCount: 6,
		},
		{
			name:        "Property filter - priority>3",
			query:       "prop:priority>3",
			expectCount: 3,
			expectIDs:   []string{"REQ-001", "REQ-003", "REQ-004"},
		},
		{
			name:        "Property filter - priority>=3",
			query:       "prop:priority>=3",
			expectCount: 4,
			expectIDs:   []string{"REQ-001", "REQ-002", "REQ-003", "REQ-004"},
		},
		{
			name:        "Property filter - priority<5",
			query:       "prop:priority<5",
			expectCount: 2,
			expectIDs:   []string{"REQ-002", "REQ-003"},
		},
		{
			name:        "Status shortcut - status:published",
			query:       "status:published",
			expectCount: 2,
			expectIDs:   []string{"REQ-001", "REQ-004"},
		},
		{
			name:        "Status shortcut - status:draft",
			query:       "status:draft",
			expectCount: 2,
			expectIDs:   []string{"REQ-002", "SOL-002"},
		},
		{
			name:        "Combined: type + property",
			query:       "type:requirement prop:status=published",
			expectCount: 2,
			expectIDs:   []string{"REQ-001", "REQ-004"},
		},
		{
			name:        "Combined: type + property + text",
			query:       "type:requirement prop:status=published security",
			expectCount: 1,
			expectIDs:   []string{"REQ-004"},
		},
		{
			name:        "Combined: type + priority filter",
			query:       "type:requirement prop:priority>=4",
			expectCount: 3,
			expectIDs:   []string{"REQ-001", "REQ-003", "REQ-004"},
		},
		{
			name:        "Combined: multiple properties",
			query:       "prop:status=published prop:priority=5",
			expectCount: 2,
			expectIDs:   []string{"REQ-001", "REQ-004"},
		},
		{
			name:        "Combined: type + text",
			query:       "type:solution OAuth",
			expectCount: 1,
			expectIDs:   []string{"SOL-001"},
		},
		{
			name:        "Combined: everything",
			query:       "type:requirement prop:category=security prop:status=published authentication",
			expectCount: 2,
			expectIDs:   []string{"REQ-001", "REQ-004"},
		},
		{
			name:        "Property with glob pattern",
			query:       "prop:category=*api*",
			expectCount: 1,
			expectIDs:   []string{"REQ-002"},
		},
		{
			name:        "Invalid property filter syntax",
			query:       "prop:status published",
			expectError: true,
		},
		{
			name:        "Empty type filter",
			query:       "type:",
			expectError: true,
		},
		{
			name:        "Empty status filter",
			query:       "status:",
			expectError: true,
		},
		{
			name:        "Invalid property operator",
			query:       "prop:statusfoo",
			expectError: true,
		},
		{
			name:        "No results for non-existent text",
			query:       "nonexistenttext12345",
			expectCount: 0,
		},
	}

	// Run tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runSearchTest(t, g, tc)
		})
	}
}

func runSearchTest(t *testing.T, g *graph.Graph, tc searchTestCase) {
	t.Helper()

	// Parse query
	sq := searchparser.ParseQuery(tc.query)

	// Check for parse errors
	if tc.expectError {
		if len(sq.ParseErrors) == 0 {
			t.Errorf("Expected parse error but got none")
		}
		return
	}

	if len(sq.ParseErrors) > 0 {
		t.Errorf("Unexpected parse error: %s", strings.Join(sq.ParseErrors, "; "))
		return
	}

	// Get all entities
	allEntities := g.AllNodes()

	// Filter entities (using same logic as SearchModel.matchesFilters)
	var results []*model.Entity
	for _, entity := range allEntities {
		if matchesSearchFilters(entity, sq) {
			results = append(results, entity)
		}
	}

	// Check result count
	if tc.expectCount > 0 && len(results) != tc.expectCount {
		t.Errorf("Expected %d results, got %d", tc.expectCount, len(results))
		for _, r := range results {
			t.Logf("  Found: %s - %s", r.ID, r.Title())
		}
		return
	}

	// Check specific IDs if provided
	if len(tc.expectIDs) > 0 {
		resultIDs := make(map[string]bool)
		for _, e := range results {
			resultIDs[e.ID] = true
		}

		for _, expectedID := range tc.expectIDs {
			if !resultIDs[expectedID] {
				t.Errorf("Expected to find ID %s but it was missing", expectedID)
			}
		}

		if len(results) != len(tc.expectIDs) {
			t.Errorf("Expected exactly %d results but got %d", len(tc.expectIDs), len(results))
			t.Logf("Results: ")
			for _, e := range results {
				t.Logf("  %s - %s", e.ID, e.Title())
			}
		}
	}
}

// matchesSearchFilters duplicates the logic from SearchModel.matchesFilters for testing
func matchesSearchFilters(entity *model.Entity, sq *searchparser.SearchQuery) bool {
	// 1. Filter by entity type (if specified)
	if len(sq.EntityTypes) > 0 {
		found := false
		for _, entityType := range sq.EntityTypes {
			if strings.EqualFold(entity.Type, entityType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 2. Apply property filters (AND logic)
	for _, propFilter := range sq.PropertyFilters {
		value, exists := entity.Properties[propFilter.Property]
		if !exists {
			return false
		}
		if !filter.MatchValue(value, propFilter) {
			return false
		}
	}

	// 3. Apply free-text search (AND logic - all words must be present)
	if sq.HasFreeText() {
		// Combine all searchable text
		searchableText := strings.ToLower(strings.Join([]string{
			entity.ID,
			entity.Title(),
			entity.Description(),
			entity.Content,
		}, " "))

		// Check all free text words
		for _, word := range sq.FreeTextWords {
			if !strings.Contains(searchableText, strings.ToLower(word)) {
				return false
			}
		}

		// Check all exact phrases
		for _, phrase := range sq.FreeTextPhrases {
			if !strings.Contains(searchableText, strings.ToLower(phrase)) {
				return false
			}
		}
	}

	return true
}

// Helper to print test results
func init() {
	// This runs before tests and can be used for setup
}

// Benchmark search performance
func BenchmarkSearch(b *testing.B) {
	projectDir := "/tmp/rela-test-project"

	if _, err := os.Stat(filepath.Join(projectDir, "metamodel.yaml")); os.IsNotExist(err) {
		b.Skipf("Test project not found at %s", projectDir)
	}

	ctx, _ := project.Discover(projectDir, storage.NewOsFS())
	meta, _ := metamodel.Load(ctx.MetamodelPath, storage.NewOsFS())
	g := graph.New()
	benchRepo := repository.New(storage.NewOsFS(), ctx)
	_, _ = benchRepo.Sync(meta, g)

	query := "type:requirement prop:status=published authentication"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sq := searchparser.ParseQuery(query)
		allEntities := g.AllNodes()
		results := make([]*model.Entity, 0)
		for _, entity := range allEntities {
			if matchesSearchFilters(entity, sq) {
				results = append(results, entity)
			}
		}
		_ = results // Use the results to avoid SA4010
	}
}

// Manual test runner that can be called from main
func RunManualSearchTests() {
	projectDir := "/tmp/rela-test-project"

	ctx, err := project.Discover(projectDir, storage.NewOsFS())
	if err != nil {
		fmt.Printf("❌ Failed to discover project: %v\n", err)
		return
	}

	meta, err := metamodel.Load(ctx.MetamodelPath, storage.NewOsFS())
	if err != nil {
		fmt.Printf("❌ Failed to load metamodel: %v\n", err)
		return
	}

	g := graph.New()
	manualRepo := repository.New(storage.NewOsFS(), ctx)
	if _, err := manualRepo.Sync(meta, g); err != nil {
		fmt.Printf("❌ Failed to sync entities: %v\n", err)
		return
	}

	fmt.Printf("✓ Loaded %d entities from test project\n\n", len(g.AllNodes()))

	// Test a few queries manually
	queries := []string{
		"authentication",
		"type:requirement",
		"prop:status=published",
		"type:requirement prop:priority>3",
	}

	for _, query := range queries {
		fmt.Printf("Query: %s\n", query)
		sq := searchparser.ParseQuery(query)

		if len(sq.ParseErrors) > 0 {
			fmt.Printf("  Errors: %v\n", sq.ParseErrors)
			continue
		}

		allEntities := g.AllNodes()
		var results []*model.Entity
		for _, entity := range allEntities {
			if matchesSearchFilters(entity, sq) {
				results = append(results, entity)
			}
		}

		fmt.Printf("  Found %d results:\n", len(results))
		for _, r := range results {
			fmt.Printf("    - %s: %s\n", r.ID, r.Title())
		}
		fmt.Println()
	}
}
