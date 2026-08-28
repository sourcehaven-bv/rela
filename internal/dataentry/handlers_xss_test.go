package dataentry

import (
	"bytes"
	htmltemplate "html/template"
	"strings"
	"testing"
)

// The entity-help fragment is assembled server-side and injected into the SPA's
// help modal, so every plain-string field it interpolates must be escaped in its
// element context. These tests pin that contract against the gosec G705 finding
// on renderHelpContent: they fail if the template ever regresses to raw
// concatenation (or if a field is switched from string to template.HTML).

// xssProbe is a payload that breaks out of both element text and an attribute
// value if it is interpolated without escaping.
const xssProbe = `<script>alert(1)</script>"><img src=x onerror=alert(2)>`

// renderHelpString renders the help fragment to a string for assertions.
func renderHelpString(
	t *testing.T, entityDesc htmltemplate.HTML, props []PropertyHelp,
	outgoing, incoming []RelationHelp, enums []EnumHelp,
) string {
	t.Helper()
	var buf bytes.Buffer
	(&App{}).renderHelpContent(&buf, entityDesc, props, outgoing, incoming, enums)
	return buf.String()
}

// assertNoRawProbe fails when the unescaped payload survived into the output.
func assertNoRawProbe(t *testing.T, got, field string) {
	t.Helper()
	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Errorf("%s: raw <script> survived escaping; output:\n%s", field, got)
	}
	if strings.Contains(got, "<img src=x onerror=alert(2)>") {
		t.Errorf("%s: raw <img onerror> survived escaping; output:\n%s", field, got)
	}
	// The payload must still be present in escaped form — otherwise the test
	// would pass trivially if the field were silently dropped.
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("%s: escaped form &lt;script&gt; not found; output:\n%s", field, got)
	}
}

// TestRenderHelpContent_EscapesPlainStringFields pins that every metamodel
// plain-string field rendered into the help fragment is HTML-escaped.
func TestRenderHelpContent_EscapesPlainStringFields(t *testing.T) {
	tests := []struct {
		name  string
		build func() string
	}{
		{"property name", func() string {
			return renderHelpString(t, "", []PropertyHelp{{Name: xssProbe, Type: "string"}}, nil, nil, nil)
		}},
		{"property type", func() string {
			return renderHelpString(t, "", []PropertyHelp{{Name: "p", Type: xssProbe}}, nil, nil, nil)
		}},
		{"outgoing relation name", func() string {
			return renderHelpString(t, "", nil, []RelationHelp{{Name: xssProbe, TargetType: "t"}}, nil, nil)
		}},
		{"outgoing relation label", func() string {
			return renderHelpString(t, "", nil, []RelationHelp{{Name: "r", Label: xssProbe, TargetType: "t"}}, nil, nil)
		}},
		{"outgoing relation target type", func() string {
			return renderHelpString(t, "", nil, []RelationHelp{{Name: "r", TargetType: xssProbe}}, nil, nil)
		}},
		{"outgoing relation cardinality", func() string {
			return renderHelpString(t, "", nil, []RelationHelp{{Name: "r", Cardinality: xssProbe}}, nil, nil)
		}},
		{"incoming relation name", func() string {
			return renderHelpString(t, "", nil, nil, []RelationHelp{{Name: xssProbe, TargetType: "t"}}, nil)
		}},
		{"enum property name", func() string {
			return renderHelpString(t, "", nil, nil, nil,
				[]EnumHelp{{Property: xssProbe, Values: []ValueHelp{{Value: "v"}}}})
		}},
		{"enum value", func() string {
			return renderHelpString(t, "", nil, nil, nil,
				[]EnumHelp{{Property: "p", Values: []ValueHelp{{Value: xssProbe}}}})
		}},
		{"enum value label", func() string {
			return renderHelpString(t, "", nil, nil, nil,
				[]EnumHelp{{Property: "p", Values: []ValueHelp{{Value: "v", Label: xssProbe}}}})
		}},
		{"transition move", func() string {
			return renderHelpString(t, "", nil, nil, nil, []EnumHelp{{Property: "p",
				Transitions: []TransitionHelp{{Move: xssProbe, From: "a", To: "b"}}}})
		}},
		{"transition from state", func() string {
			return renderHelpString(t, "", nil, nil, nil, []EnumHelp{{Property: "p",
				Transitions: []TransitionHelp{{Move: "m", From: xssProbe, To: "b"}}}})
		}},
		{"transition to state", func() string {
			return renderHelpString(t, "", nil, nil, nil, []EnumHelp{{Property: "p",
				Transitions: []TransitionHelp{{Move: "m", From: "a", To: xssProbe}}}})
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertNoRawProbe(t, tc.build(), tc.name)
		})
	}
}

// TestRenderHelpContent_MermaidDiagramEscaped pins that the generated mermaid
// state-diagram source — which embeds state names — is escaped inside the
// <pre class="mermaid"> block rather than injected as markup.
func TestRenderHelpContent_MermaidDiagramEscaped(t *testing.T) {
	got := renderHelpString(t, "", nil, nil, nil, []EnumHelp{{
		Property:    "status",
		Initial:     "todo",
		Transitions: []TransitionHelp{{Move: "go", From: "todo", To: `</pre><script>alert(1)</script>`}},
	}})

	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Errorf("mermaid block leaked raw markup; output:\n%s", got)
	}
	if !strings.Contains(got, `<pre class="mermaid">`) {
		t.Errorf("expected a mermaid pre block; output:\n%s", got)
	}
}

// TestRenderHelpContent_DescriptionIsTrustedMarkup documents the one deliberate
// raw-markup seam: Description / Help are template.HTML holding goldmark output
// built from OPERATOR-authored metamodel.yaml prose (simpleMarkdownToHTML,
// helpers.go). That converter runs goldmark with html.WithUnsafe(), so it does
// NOT sanitize — the safety argument is the trust of the source file, which no
// HTTP/MCP/Lua write path can modify. This test pins that the seam is
// intentional and, crucially, that it is reachable ONLY via template.HTML: if
// these fields were ever changed to plain strings the markup would be escaped
// and this test would fail, forcing a deliberate re-review.
func TestRenderHelpContent_DescriptionIsTrustedMarkup(t *testing.T) {
	const markup = `<em>emphasis</em>`

	t.Run("entity description passes markup through", func(t *testing.T) {
		got := renderHelpString(t, htmltemplate.HTML(markup), nil, nil, nil, nil)
		if !strings.Contains(got, markup) {
			t.Errorf("entity description markup was escaped; output:\n%s", got)
		}
	})

	t.Run("property description passes markup through", func(t *testing.T) {
		got := renderHelpString(t, "",
			[]PropertyHelp{{Name: "p", Description: htmltemplate.HTML(markup)}}, nil, nil, nil)
		if !strings.Contains(got, markup) {
			t.Errorf("property description markup was escaped; output:\n%s", got)
		}
	})
}
