package mermaid_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/mermaid"
)

func TestStateDiagram_EntryAndEdges(t *testing.T) {
	t.Parallel()
	got := mermaid.StateDiagram("todo", []mermaid.Transition{
		{From: "todo", To: "doing", Label: "start"},
		{From: "doing", To: "done", Label: "finish"},
	})
	// Header, alias declarations in first-seen order, entry arrow, labeled edges.
	for _, want := range []string{
		"stateDiagram-v2",
		`state "todo" as s0`,
		`state "doing" as s1`,
		`state "done" as s2`,
		"[*] --> s0",
		"s0 --> s1: start",
		"s1 --> s2: finish",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diagram missing %q\n%s", want, got)
		}
	}
}

func TestStateDiagram_NoInitial_NoEntryArrow(t *testing.T) {
	t.Parallel()
	got := mermaid.StateDiagram("", []mermaid.Transition{{From: "a", To: "b", Label: "go"}})
	if strings.Contains(got, "[*]") {
		t.Errorf("no initial → no entry arrow, got:\n%s", got)
	}
	if !strings.Contains(got, "s0 --> s1: go") {
		t.Errorf("expected the edge, got:\n%s", got)
	}
}

func TestStateDiagram_LabelEqualToTarget_IsBare(t *testing.T) {
	t.Parallel()
	// A move labeled the same as its target value adds no information → bare edge.
	got := mermaid.StateDiagram("", []mermaid.Transition{{From: "a", To: "done", Label: "done"}})
	if strings.Contains(got, ": done") {
		t.Errorf("label == target should render bare, got:\n%s", got)
	}
}

// AC5 / AC11: a value or label containing mermaid-breaking characters must NOT
// inject syntax. States are referenced by synthetic id; the raw text appears
// only inside a quoted alias, and labels are newline-flattened.
func TestStateDiagram_InjectionSafe(t *testing.T) {
	t.Parallel()
	got := mermaid.StateDiagram("in progress", []mermaid.Transition{
		{From: "in progress", To: "a --> evil", Label: "click\n[*] --> hacked"},
	})
	// The dangerous value is aliased to a synthetic id, never spliced as syntax.
	if !strings.Contains(got, `as s0`) || !strings.Contains(got, `as s1`) {
		t.Errorf("states must be aliased to synthetic ids, got:\n%s", got)
	}
	// The injected `[*] --> hacked` may survive only as label TEXT (right of the
	// `: `), never as its own statement. Every edge/entry line's SOURCE (left of
	// the first -->) must be [*] or a synthetic id — a leaked raw value fails here.
	for line := range strings.SplitSeq(strings.TrimSpace(got), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "stateDiagram-v2" || strings.HasPrefix(trimmed, `state "`) {
			continue
		}
		if strings.Contains(trimmed, "-->") {
			left := strings.TrimSpace(strings.SplitN(trimmed, "-->", 2)[0])
			if left != "[*]" && !isSyntheticStateID(left) {
				t.Errorf("edge line has a non-synthetic source %q: %q", left, trimmed)
			}
		}
	}
	// The newline in the label must be flattened (no bare newline mid-diagram).
	if strings.Contains(got, "click\n") {
		t.Errorf("label newline was not flattened, got:\n%s", got)
	}
}

func isSyntheticStateID(s string) bool {
	if !strings.HasPrefix(s, "s") {
		return false
	}
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 1
}

func TestStateDiagram_SkipsEmptyEndpoints(t *testing.T) {
	t.Parallel()
	got := mermaid.StateDiagram("a", []mermaid.Transition{
		{From: "a", To: "", Label: "x"}, // missing target — undrawable, skip
		{From: "a", To: "b", Label: "ok"},
	})
	if !strings.Contains(got, "s0 --> s1: ok") {
		t.Errorf("valid edge missing, got:\n%s", got)
	}
	if strings.Contains(got, ": x") {
		t.Errorf("edge with empty endpoint should be skipped, got:\n%s", got)
	}
}

func TestGraph_NodesEdgesDedup(t *testing.T) {
	t.Parallel()
	got := mermaid.Graph(
		[]mermaid.Node{
			{Key: "verwerking", Text: "verwerking"},
			{Key: "doel", Text: "verwerkingsdoel"},
			{Key: "verwerking", Text: "verwerking"}, // duplicate key → collapsed
		},
		[]mermaid.Edge{
			{FromKey: "verwerking", ToKey: "doel", Label: "heeft_doel"},
			{FromKey: "verwerking", ToKey: "doel", Label: "heeft_doel"}, // dup edge → collapsed
		},
	)
	if strings.Count(got, `n0["verwerking"]`) != 1 {
		t.Errorf("node declared once, got:\n%s", got)
	}
	if c := strings.Count(got, "heeft_doel"); c != 1 {
		t.Errorf("duplicate edge should collapse to 1, got %d:\n%s", c, got)
	}
	if !strings.Contains(got, "graph LR") {
		t.Errorf("expected LR flow header, got:\n%s", got)
	}
}

func TestGraph_SkipsDanglingEdge(t *testing.T) {
	t.Parallel()
	got := mermaid.Graph(
		[]mermaid.Node{{Key: "a", Text: "a"}},
		[]mermaid.Edge{{FromKey: "a", ToKey: "missing", Label: "x"}},
	)
	if strings.Contains(got, "x") {
		t.Errorf("edge to undeclared node should be skipped, got:\n%s", got)
	}
}

func TestGraph_InjectionSafeNodeText(t *testing.T) {
	t.Parallel()
	got := mermaid.Graph(
		[]mermaid.Node{{Key: "k", Text: "evil\"]-->x[\"pwned"}},
		nil,
	)
	// Node text is quoted; a newline would be flattened. The raw text with a
	// newline must not appear verbatim outside the quoted form.
	if strings.Contains(got, "pwned\n") {
		t.Errorf("node text newline not flattened, got:\n%s", got)
	}
}

func TestLabel_FlattensNewlines(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a\nb":     "a b",
		"a\r\nb":   "a b",
		"a\rb":     "a b",
		"  x  ":    "x",
		"plain":    "plain",
		"a\n\n b ": "a   b", // two newlines → two spaces, plus the space before b
	}
	for in, want := range cases {
		if got := mermaid.Label(in); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
}
