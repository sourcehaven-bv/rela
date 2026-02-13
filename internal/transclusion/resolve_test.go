package transclusion

import (
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func setupTestGraph() *graph.Graph {
	g := graph.New()

	// Add test entities
	g.AddNode(&model.Entity{
		ID:      "REQ-001",
		Type:    "requirement",
		Content: "# Requirements\n\nThis is the requirements document.\n\n## Rationale\n\nWhy we need this.",
	})

	g.AddNode(&model.Entity{
		ID:      "REQ-002",
		Type:    "requirement",
		Content: "# Another Requirement\n\nSee also: ![[REQ-001#Rationale]]",
	})

	g.AddNode(&model.Entity{
		ID:      "CIRCULAR-A",
		Type:    "test",
		Content: "A references B: ![[CIRCULAR-B]]",
	})

	g.AddNode(&model.Entity{
		ID:      "CIRCULAR-B",
		Type:    "test",
		Content: "B references A: ![[CIRCULAR-A]]",
	})

	g.AddNode(&model.Entity{
		ID:      "NESTED-1",
		Type:    "test",
		Content: "Level 1: ![[NESTED-2]]",
	})

	g.AddNode(&model.Entity{
		ID:      "NESTED-2",
		Type:    "test",
		Content: "Level 2: ![[NESTED-3]]",
	})

	g.AddNode(&model.Entity{
		ID:      "NESTED-3",
		Type:    "test",
		Content: "Level 3 content",
	})

	g.AddNode(&model.Entity{
		ID:      "DIAMOND-A",
		Type:    "test",
		Content: "A includes B and C: ![[DIAMOND-B]] and ![[DIAMOND-C]]",
	})

	g.AddNode(&model.Entity{
		ID:      "DIAMOND-B",
		Type:    "test",
		Content: "B includes D: ![[DIAMOND-D]]",
	})

	g.AddNode(&model.Entity{
		ID:      "DIAMOND-C",
		Type:    "test",
		Content: "C includes D: ![[DIAMOND-D]]",
	})

	g.AddNode(&model.Entity{
		ID:      "DIAMOND-D",
		Type:    "test",
		Content: "D is shared",
	})

	g.AddNode(&model.Entity{
		ID:      "WITH-COMMENTS",
		Type:    "test",
		Content: "Before <!-- comment --> After",
	})

	return g
}

func TestResolver_Resolve(t *testing.T) {
	g := setupTestGraph()
	r := NewResolver(g)

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "no transclusions",
			content: "Just some text.",
			want:    "Just some text.",
		},
		{
			name:    "simple transclusion",
			content: "See: ![[REQ-001]]",
			want:    "See: # Requirements\n\nThis is the requirements document.\n\n## Rationale\n\nWhy we need this.",
		},
		{
			name:    "section transclusion",
			content: "Rationale: ![[REQ-001#Rationale]]",
			want:    "Rationale: ## Rationale\n\nWhy we need this.",
		},
		{
			name:    "missing entity",
			content: "![[NONEXISTENT]]",
			wantErr: true,
		},
		{
			name:    "missing section",
			content: "![[REQ-001#Nonexistent]]",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.Resolve(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Resolve() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestResolver_CircularDetection(t *testing.T) {
	g := setupTestGraph()
	r := NewResolver(g)

	_, err := r.ResolveEntity("CIRCULAR-A")
	if err == nil {
		t.Error("expected circular transclusion error")
		return
	}

	var circErr *CircularTransclusionError
	if !errors.As(err, &circErr) {
		t.Errorf("expected CircularTransclusionError, got %T", err)
		return
	}

	// Check that the chain shows the cycle
	if len(circErr.Chain) < 2 {
		t.Errorf("expected chain with at least 2 elements, got %v", circErr.Chain)
	}
}

func TestResolver_NestedTransclusions(t *testing.T) {
	g := setupTestGraph()
	r := NewResolver(g)

	got, err := r.ResolveEntity("NESTED-1")
	if err != nil {
		t.Fatalf("ResolveEntity() error = %v", err)
	}

	// Should contain content from all nested levels
	if got != "Level 1: Level 2: Level 3 content" {
		t.Errorf("ResolveEntity() =\n%q\nwant content from all levels", got)
	}
}

func TestResolver_DiamondPattern(t *testing.T) {
	g := setupTestGraph()
	r := NewResolver(g)

	// Diamond pattern should work (D is included from both B and C)
	got, err := r.ResolveEntity("DIAMOND-A")
	if err != nil {
		t.Fatalf("ResolveEntity() error = %v", err)
	}

	// D should appear twice (once from B, once from C)
	expected := "A includes B and C: B includes D: D is shared and C includes D: D is shared"
	if got != expected {
		t.Errorf("ResolveEntity() =\n%q\nwant\n%q", got, expected)
	}
}

func TestResolver_MaxDepth(t *testing.T) {
	g := setupTestGraph()
	r := NewResolver(g).WithMaxDepth(1)

	got, err := r.ResolveEntity("NESTED-1")
	if err != nil {
		t.Fatalf("ResolveEntity() error = %v", err)
	}

	// With max depth 1, should only resolve one level
	// The nested transclusion ![[NESTED-3]] should remain unresolved
	if got != "Level 1: Level 2: ![[NESTED-3]]" {
		t.Errorf("ResolveEntity() with maxDepth=1 =\n%q", got)
	}
}

func TestResolver_RenderEntity(t *testing.T) {
	g := setupTestGraph()
	r := NewResolver(g)

	t.Run("with frontmatter", func(t *testing.T) {
		got, err := r.RenderEntity("REQ-001", RenderOptions{IncludeFrontmatter: true})
		if err != nil {
			t.Fatalf("RenderEntity() error = %v", err)
		}

		if !containsAll(got, "---", "id: REQ-001", "type: requirement") {
			t.Errorf("RenderEntity() missing frontmatter elements:\n%s", got)
		}
	})

	t.Run("strip comments", func(t *testing.T) {
		got, err := r.RenderEntity("WITH-COMMENTS", RenderOptions{StripComments: true})
		if err != nil {
			t.Fatalf("RenderEntity() error = %v", err)
		}

		if got != "Before  After" {
			t.Errorf("RenderEntity() =\n%q\nwant comment stripped", got)
		}
	})

	t.Run("missing entity", func(t *testing.T) {
		_, err := r.RenderEntity("NONEXISTENT", DefaultRenderOptions())
		if err == nil {
			t.Error("expected error for missing entity")
		}
	})
}

func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if !containsStr(s, sub) {
			return false
		}
	}
	return true
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && (s[:len(substr)] == substr || containsStr(s[1:], substr)))
}
