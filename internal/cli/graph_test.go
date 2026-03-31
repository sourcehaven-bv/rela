package cli

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

func setupGraphTestGraph() {
	g = graph.New()
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
				Color:    "#E3F2FD",
			},
			"decision": {
				Label:    "Decision",
				IDPrefix: "DEC-",
				Color:    "#FFF3E0",
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "Implements",
				From:  []string{"decision"},
				To:    []string{"requirement"},
			},
		},
	}
	ws = workspace.NewForTest(g, meta)
	out = output.New(output.FormatTable)

	// Add test entities
	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").
		With("title", "First Requirement").
		Build())

	g.AddNode(testutil.EntityFor(meta, "requirement").
		ID("REQ-002").
		With("title", "Second Requirement").
		Build())

	g.AddNode(testutil.EntityFor(meta, "decision").
		ID("DEC-001").
		With("title", "Important Decision").
		Build())

	// Add test relations
	g.AddEdge(testutil.NewRelation("DEC-001", "implements", "REQ-001").Build())
}

func TestGenerateDOT_BasicOutput(t *testing.T) {
	setupGraphTestGraph()

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	// Should start with digraph
	if !strings.HasPrefix(dot, "digraph architecture {") {
		t.Errorf("DOT should start with 'digraph architecture {', got:\n%s", dot)
	}

	// Should end with closing brace
	if !strings.HasSuffix(strings.TrimSpace(dot), "}") {
		t.Errorf("DOT should end with '}', got:\n%s", dot)
	}

	// Should contain rankdir
	if !strings.Contains(dot, "rankdir=TB") {
		t.Error("DOT should contain 'rankdir=TB' by default")
	}

	// Should contain node style
	if !strings.Contains(dot, "node [shape=box, style=filled]") {
		t.Error("DOT should contain node style definition")
	}
}

func TestGenerateDOT_ContainsEntities(t *testing.T) {
	setupGraphTestGraph()

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	// Should contain entity IDs
	if !strings.Contains(dot, `"REQ-001"`) {
		t.Error("DOT should contain REQ-001")
	}
	if !strings.Contains(dot, `"REQ-002"`) {
		t.Error("DOT should contain REQ-002")
	}
	if !strings.Contains(dot, `"DEC-001"`) {
		t.Error("DOT should contain DEC-001")
	}

	// Should contain entity titles in labels
	if !strings.Contains(dot, "First Requirement") {
		t.Error("DOT should contain 'First Requirement' title")
	}
}

func TestGenerateDOT_ContainsEdges(t *testing.T) {
	setupGraphTestGraph()

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	// Should contain edge definition
	if !strings.Contains(dot, `"DEC-001" -> "REQ-001"`) {
		t.Error("DOT should contain edge from DEC-001 to REQ-001")
	}

	// Should contain edge label
	if !strings.Contains(dot, `label="implements"`) {
		t.Error("DOT should contain 'implements' label on edge")
	}
}

func TestGenerateDOT_GroupsByType(t *testing.T) {
	setupGraphTestGraph()

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	// Should contain subgraph clusters
	if !strings.Contains(dot, "subgraph cluster_requirement") {
		t.Error("DOT should contain 'subgraph cluster_requirement'")
	}
	if !strings.Contains(dot, "subgraph cluster_decision") {
		t.Error("DOT should contain 'subgraph cluster_decision'")
	}
}

func TestGenerateDOT_AppliesColors(t *testing.T) {
	setupGraphTestGraph()

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	// Should contain colors from metamodel
	if !strings.Contains(dot, `fillcolor="#E3F2FD"`) {
		t.Error("DOT should contain requirement color #E3F2FD")
	}
	if !strings.Contains(dot, `fillcolor="#FFF3E0"`) {
		t.Error("DOT should contain decision color #FFF3E0")
	}
}

func TestGenerateDOT_DirectionLR(t *testing.T) {
	setupGraphTestGraph()

	// Set direction to left-right
	graphDirection = "lr"
	defer func() { graphDirection = "" }()

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	if !strings.Contains(dot, "rankdir=LR") {
		t.Error("DOT should contain 'rankdir=LR' when direction is lr")
	}
}

func TestGenerateDOT_EmptyGraph(t *testing.T) {
	g = graph.New()
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{},
	}
	ws = workspace.NewForTest(g, meta)

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	// Should still be valid DOT
	if !strings.HasPrefix(dot, "digraph architecture {") {
		t.Errorf("DOT should start with 'digraph architecture {', got:\n%s", dot)
	}
	if !strings.HasSuffix(strings.TrimSpace(dot), "}") {
		t.Errorf("DOT should end with '}', got:\n%s", dot)
	}
}

func TestGenerateDOT_EntityWithoutTitle(t *testing.T) {
	g = graph.New()
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"component": {
				Label:    "Component",
				IDPrefix: "CMP-",
			},
		},
	}
	ws = workspace.NewForTest(g, meta)

	// Add entity without title
	g.AddNode(testutil.Entity("component").ID("CMP-001").Build())

	entities := g.AllNodes()
	edges := g.AllEdges()

	dot := generateDOT(entities, edges)

	// Should use ID as label when no title
	if !strings.Contains(dot, `label="CMP-001"`) {
		t.Error("DOT should use entity ID as label when no title is set")
	}
}

func TestEscapeLabel_Basic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string unchanged",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "escapes quotes",
			input: `Say "Hello"`,
			want:  `Say \"Hello\"`,
		},
		{
			name:  "escapes newlines",
			input: "Line1\nLine2",
			want:  `Line1\nLine2`,
		},
		{
			name:  "escapes both quotes and newlines",
			input: "He said:\n\"Hello\"",
			want:  `He said:\n\"Hello\"`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeLabel(tt.input)
			if got != tt.want {
				t.Errorf("escapeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeLabel_Truncation(t *testing.T) {
	// maxLabelLen is 40
	longString := "This is a very long label that exceeds the maximum allowed length for graph labels"

	got := escapeLabel(longString)

	// Should be truncated to maxLabelLen
	if len(got) > maxLabelLen {
		t.Errorf("escapeLabel should truncate to %d chars, got %d", maxLabelLen, len(got))
	}

	// Should end with "..."
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated label should end with '...', got %q", got)
	}
}

func TestEscapeLabel_ExactMaxLength(t *testing.T) {
	// String exactly at maxLabelLen should not be truncated
	exactString := strings.Repeat("a", maxLabelLen)

	got := escapeLabel(exactString)

	if got != exactString {
		t.Errorf("string at exact max length should not be modified, got %q", got)
	}
}
