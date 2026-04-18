package cli

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// graph_test.go covers DOT generation — the sole CLI formatting
// responsibility of `rela graph`. Store iteration is provided by the
// store layer; here we only verify that `generateDOT` renders a
// well-formed, correctly-populated DOT document.

func setupGraphTestGraph() {
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
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "requirement").
		ID("REQ-001").With("title", "First Requirement"))
	seeder.addEntity(testutil.EntityFor(meta, "requirement").
		ID("REQ-002").With("title", "Second Requirement"))
	seeder.addEntity(testutil.EntityFor(meta, "decision").
		ID("DEC-001").With("title", "Important Decision"))
	seeder.addRelation("DEC-001", "implements", "REQ-001")
	applySeeder(seeder)
	out = output.New(output.FormatTable)
}

// TestGenerateDOT is the canonical test for DOT rendering. One fixture,
// one render, all invariants asserted in a single place.
func TestGenerateDOT(t *testing.T) {
	setupGraphTestGraph()

	dot := generateDOT(fixtureAllEntities(), fixtureAllRelations())

	// Structural invariants.
	if !strings.HasPrefix(dot, "digraph architecture {") {
		t.Errorf("DOT should start with 'digraph architecture {', got:\n%s", dot)
	}
	if !strings.HasSuffix(strings.TrimSpace(dot), "}") {
		t.Errorf("DOT should end with '}', got:\n%s", dot)
	}
	if !strings.Contains(dot, "rankdir=TB") {
		t.Error("DOT should contain 'rankdir=TB' by default")
	}
	if !strings.Contains(dot, "node [shape=box, style=filled]") {
		t.Error("DOT should contain node style definition")
	}

	// Entities + titles + colors.
	for _, want := range []string{
		`"REQ-001"`, `"REQ-002"`, `"DEC-001"`,
		"First Requirement",
		`fillcolor="#E3F2FD"`, // requirement color
		`fillcolor="#FFF3E0"`, // decision color
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT missing %q", want)
		}
	}

	// Edge rendering.
	if !strings.Contains(dot, `"DEC-001" -> "REQ-001"`) {
		t.Error("DOT should contain edge from DEC-001 to REQ-001")
	}
	if !strings.Contains(dot, `label="implements"`) {
		t.Error("DOT should contain 'implements' label on edge")
	}

	// Type-based clustering.
	if !strings.Contains(dot, "subgraph cluster_requirement") {
		t.Error("DOT should contain 'subgraph cluster_requirement'")
	}
	if !strings.Contains(dot, "subgraph cluster_decision") {
		t.Error("DOT should contain 'subgraph cluster_decision'")
	}
}

func TestGenerateDOT_DirectionLR(t *testing.T) {
	setupGraphTestGraph()

	graphDirection = "lr"
	defer func() { graphDirection = "" }()

	dot := generateDOT(fixtureAllEntities(), fixtureAllRelations())

	if !strings.Contains(dot, "rankdir=LR") {
		t.Error("DOT should contain 'rankdir=LR' when direction is lr")
	}
}

func TestGenerateDOT_EmptyGraph(t *testing.T) {
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{},
	}
	applySeeder(newStoreSeeder(meta))

	dot := generateDOT(fixtureAllEntities(), fixtureAllRelations())

	if !strings.HasPrefix(dot, "digraph architecture {") {
		t.Errorf("DOT should start with 'digraph architecture {', got:\n%s", dot)
	}
	if !strings.HasSuffix(strings.TrimSpace(dot), "}") {
		t.Errorf("DOT should end with '}', got:\n%s", dot)
	}
}

func TestGenerateDOT_EntityWithoutTitle(t *testing.T) {
	meta = &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"component": {Label: "Component", IDPrefix: "CMP-"},
		},
	}
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.Entity("component").ID("CMP-001"))
	applySeeder(seeder)

	dot := generateDOT(fixtureAllEntities(), fixtureAllRelations())

	if !strings.Contains(dot, `label="CMP-001"`) {
		t.Error("DOT should use entity ID as label when no title is set")
	}
}

func TestEscapeLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple string unchanged", "Hello World", "Hello World"},
		{"escapes quotes", `Say "Hello"`, `Say \"Hello\"`},
		{"escapes newlines", "Line1\nLine2", `Line1\nLine2`},
		{"escapes both quotes and newlines", "He said:\n\"Hello\"", `He said:\n\"Hello\"`},
		{"empty string", "", ""},
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
	longString := "This is a very long label that exceeds the maximum allowed length for graph labels"

	got := escapeLabel(longString)

	if len(got) > maxLabelLen {
		t.Errorf("escapeLabel should truncate to %d chars, got %d", maxLabelLen, len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated label should end with '...', got %q", got)
	}

	// String exactly at maxLabelLen should not be truncated.
	exactString := strings.Repeat("a", maxLabelLen)
	if got := escapeLabel(exactString); got != exactString {
		t.Errorf("string at exact max length should not be modified, got %q", got)
	}
}
