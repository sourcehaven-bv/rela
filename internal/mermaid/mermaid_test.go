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

// Label must neutralize the characters that break OUT of a mermaid quoted
// label — double quotes and angle brackets — using mermaid's entity forms
// (NOT backslash, which mermaid does not honor).
func TestLabel_EscapesQuotesAndBrackets(t *testing.T) {
	t.Parallel()
	got := mermaid.Label(`a"] --> x["b`)
	if strings.Contains(got, `"`) {
		t.Errorf("bare double-quote must be entity-escaped, got %q", got)
	}
	if !strings.Contains(got, "#quot;") {
		t.Errorf("expected #quot; entity, got %q", got)
	}
	if strings.ContainsAny(mermaid.Label("a<b>c"), "<>") {
		t.Errorf("angle brackets must be escaped, got %q", mermaid.Label("a<b>c"))
	}
}

// A node whose text contains a double quote must not break out of the
// bracketed quoted label (the regression the reviewer flagged: %q emits \",
// which mermaid does not treat as an escape).
func TestGraph_QuoteInNodeTextDoesNotBreakOut(t *testing.T) {
	t.Parallel()
	got := mermaid.Graph([]mermaid.Node{{Key: "k", Text: `evil"] --> hax["pwned`}}, nil)
	// The safety property: the label text sits between exactly two delimiter
	// quotes. Any internal quote is entity-escaped, so the `] --> [` sequence is
	// inert label text, not structure — it cannot break out of the "…" label.
	if strings.Count(got, `"`) != 2 {
		t.Errorf("node label must contain exactly the 2 delimiter quotes, got:\n%s", got)
	}
	if strings.Contains(got, `"]`) && !strings.Contains(got, `#quot;]`) {
		t.Errorf("a raw quote precedes a bracket — possible breakout:\n%s", got)
	}
}

// Likewise a state value or transition label carrying a double quote.
func TestStateDiagram_QuoteInValueDoesNotBreakOut(t *testing.T) {
	t.Parallel()
	got := mermaid.StateDiagram(`o"pen`, []mermaid.Transition{{From: `o"pen`, To: "closed", Label: `cl"ose`}})
	for line := range strings.SplitSeq(got, "\n") {
		// Each state alias line has exactly two delimiter quotes; no other line
		// may contain a raw quote.
		if strings.HasPrefix(strings.TrimSpace(line), "state ") {
			if strings.Count(line, `"`) != 2 {
				t.Errorf("state alias must have exactly 2 delimiter quotes: %q", line)
			}
			continue
		}
		if strings.Contains(line, `"`) {
			t.Errorf("non-alias line leaked a raw quote: %q", line)
		}
	}
}
