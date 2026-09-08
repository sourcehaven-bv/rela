package markdown_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

func TestAppendToSection(t *testing.T) {
	tests := []struct {
		name    string
		content string
		section string
		line    string
		want    string
	}{
		{
			name:    "appends to an existing section at end of document",
			content: "## Notes\n\nfirst",
			section: "Notes",
			line:    "- second",
			want:    "## Notes\n\nfirst\n- second",
		},
		{
			name:    "appends before the next same-level heading",
			content: "## Notes\n\nfirst\n\n## Other\n\nkeep",
			section: "Notes",
			line:    "- second",
			want:    "## Notes\n\nfirst\n- second\n\n## Other\n\nkeep",
		},
		{
			name:    "appends before a higher-level heading",
			content: "### Deep\n\nfirst\n\n## Shallow\n\nkeep",
			section: "Deep",
			line:    "- second",
			want:    "### Deep\n\nfirst\n- second\n\n## Shallow\n\nkeep",
		},
		{
			name:    "a nested subsection belongs to its parent",
			content: "## Notes\n\nintro\n\n### Detail\n\ninner\n\n## Other\n\nkeep",
			section: "Notes",
			line:    "- appended",
			want:    "## Notes\n\nintro\n\n### Detail\n\ninner\n- appended\n\n## Other\n\nkeep",
		},
		{
			name:    "missing section is created at end of document",
			content: "# Title\n\nbody",
			section: "Notifications",
			line:    "- alert",
			want:    "# Title\n\nbody\n\n## Notifications\n\n- alert",
		},
		{
			name:    "missing section on empty content",
			content: "",
			section: "Notifications",
			line:    "- alert",
			want:    "## Notifications\n\n- alert",
		},
		{
			name:    "match is case-insensitive and whitespace-tolerant",
			content: "##   notifications  \n\nfirst",
			section: "Notifications",
			line:    "- second",
			want:    "##   notifications  \n\nfirst\n- second",
		},
		{
			name:    "empty section name appends to the body",
			content: "# Title\n\nbody",
			section: "",
			line:    "- appended",
			want:    "# Title\n\nbody\n\n- appended",
		},
		{
			// A trailing newline is part of the document's existing shape, so
			// the append preserves it rather than normalizing it away.
			name:    "section running to end of document with trailing newline",
			content: "## Notes\n\nfirst\n",
			section: "Notes",
			line:    "- second",
			want:    "## Notes\n\nfirst\n- second\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := markdown.AppendToSection(tc.content, tc.section, tc.line)
			if got != tc.want {
				t.Errorf("AppendToSection()\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestAppendToSection_IgnoresFencedCodeHeadings pins that a `#` line inside a
// fenced code block is not treated as a heading — neither as a match target nor
// as a section terminator. A line scan would get both wrong, which is why the
// implementation parses.
func TestAppendToSection_IgnoresFencedCodeHeadings(t *testing.T) {
	content := strings.Join([]string{
		"## Notes",
		"",
		"```sh",
		"## Other",
		"echo hi",
		"```",
		"",
		"real content",
		"",
		"## Other",
		"",
		"keep",
	}, "\n")

	got := markdown.AppendToSection(content, "Notes", "- appended")

	// The appended line must land after "real content" (still inside Notes),
	// NOT after the fenced "## Other".
	want := strings.Join([]string{
		"## Notes",
		"",
		"```sh",
		"## Other",
		"echo hi",
		"```",
		"",
		"real content",
		"- appended",
		"",
		"## Other",
		"",
		"keep",
	}, "\n")
	if got != want {
		t.Errorf("fenced-code heading mishandled\n got %q\nwant %q", got, want)
	}
}

// TestAppendToSection_RepeatedAppendsAccumulateInOrder is the property the
// webhook append relies on: each delivery adds one line, in arrival order, with
// no drift of blank-line separators and no duplication of the heading.
func TestAppendToSection_RepeatedAppendsAccumulateInOrder(t *testing.T) {
	content := "# Incident\n\nSummary."
	for _, line := range []string{"- first", "- second", "- third"} {
		content = markdown.AppendToSection(content, "Notifications", line)
	}

	want := "# Incident\n\nSummary.\n\n## Notifications\n\n- first\n- second\n- third"
	if content != want {
		t.Errorf("repeated appends\n got %q\nwant %q", content, want)
	}
	if n := strings.Count(content, "## Notifications"); n != 1 {
		t.Errorf("heading duplicated %d times, want exactly 1", n)
	}
}

// TestAppendToSection_SetextHeading covers the alternate heading syntax, whose
// recorded source segment is the TEXT line rather than a `#` prefix.
func TestAppendToSection_SetextHeading(t *testing.T) {
	content := "Notes\n-----\n\nfirst\n\nOther\n-----\n\nkeep"
	got := markdown.AppendToSection(content, "Notes", "- second")
	want := "Notes\n-----\n\nfirst\n- second\n\nOther\n-----\n\nkeep"
	if got != want {
		t.Errorf("setext heading\n got %q\nwant %q", got, want)
	}
}

// TestAppendToSection_PreservesOtherContent asserts the transform is additive:
// every original line survives, in order, with exactly one line added.
func TestAppendToSection_PreservesOtherContent(t *testing.T) {
	content := "# T\n\nintro\n\n## A\n\naaa\n\n## B\n\nbbb"
	got := markdown.AppendToSection(content, "A", "- added")

	before := strings.Split(content, "\n")
	after := strings.Split(got, "\n")
	if len(after) != len(before)+1 {
		t.Fatalf("line count = %d, want %d (exactly one line added)", len(after), len(before)+1)
	}
	// Every original line must still appear, in relative order.
	idx := 0
	for _, orig := range before {
		for idx < len(after) && after[idx] != orig {
			idx++
		}
		if idx >= len(after) {
			t.Fatalf("original line %q lost or reordered in %q", orig, got)
		}
		idx++
	}
}
