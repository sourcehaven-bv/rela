package dataentry

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestRenderEntityMarkdown_CarriesID pins the property that makes the {id}
// command-line placeholder unnecessary: the markdown handed to a renderer as
// {in} always carries the id in its frontmatter (TKT-QGHNVA).
func TestRenderEntityMarkdown_CarriesID(t *testing.T) {
	e := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "Fix the thing", "status": "open"},
		Content:    "Body text.",
	}

	got, err := renderEntityMarkdown(e)
	if err != nil {
		t.Fatalf("renderEntityMarkdown: %v", err)
	}

	if !strings.Contains(got, "id: TKT-001") {
		t.Errorf("serialized entity must carry `id:` in frontmatter, got:\n%s", got)
	}
	if !strings.Contains(got, "type: ticket") {
		t.Errorf("serialized entity must carry `type:`, got:\n%s", got)
	}
	if !strings.Contains(got, "Body text.") {
		t.Errorf("serialized entity must carry the body, got:\n%s", got)
	}
}

// TestRenderEntityMarkdown_HostileIDIsInertData is the regression test for the
// original finding: an id that would read as an option flag on a command line
// is now only ever file CONTENT, never an argument.
//
// The id still round-trips verbatim — this change removes the injection
// channel, it does not sanitize the value.
func TestRenderEntityMarkdown_HostileIDIsInertData(t *testing.T) {
	for _, id := range []string{"-rf", "-oevil", "--output=x"} {
		e := &entity.Entity{ID: id, Type: "ticket"}

		got, err := renderEntityMarkdown(e)
		if err != nil {
			t.Fatalf("renderEntityMarkdown(%q): %v", id, err)
		}
		if !strings.Contains(got, id) {
			t.Errorf("id %q should survive into the {in} file verbatim, got:\n%s", id, got)
		}
	}
}

// TestExecuteCommand_NoShellInterpretation is the load-bearing assertion: the
// command is an argv array run directly, so shell metacharacters in an
// argument are literal text rather than syntax.
//
// If someone reintroduces `sh -c`, the echoed argument would be interpreted
// (the `;` would terminate a command, the backticks would substitute) and this
// test fails.
func TestExecuteCommand_NoShellInterpretation(t *testing.T) {
	s := &documentService{}

	const hostile = "a;b`whoami`$(id)"
	out, err := s.executeCommand(t.Context(), []string{"echo", hostile}, nil, 0)
	if err != nil {
		t.Skipf("echo unavailable or sandbox refused in this environment: %v", err)
	}

	if strings.TrimSpace(out) != hostile {
		t.Errorf("argument must reach the program uninterpreted:\n got: %q\nwant: %q",
			strings.TrimSpace(out), hostile)
	}
}

// TestExecuteCommand_EmptyCommandIsRejected covers the guard that keeps an
// empty argv from reaching the runner as a confusing lower-level error.
func TestExecuteCommand_EmptyCommandIsRejected(t *testing.T) {
	s := &documentService{}
	if _, err := s.executeCommand(t.Context(), nil, nil, 0); err == nil {
		t.Fatal("expected an error for an empty command")
	}
}
