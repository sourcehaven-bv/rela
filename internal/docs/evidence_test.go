package docs

import (
	"context"
	"strings"
	"testing"
)

// buildOut builds a manual and returns its rendered markdown.
func buildOut(t *testing.T, src string, opts Options) string {
	t.Helper()
	out, err := Build(context.Background(), src, opts)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	return out
}

// TestAssertionsRenderEvidence is the regression test for the defect that
// motivated this: assertions used to emit NOTHING, so a manual reading
//
//	The reader's world holds only what has been published:
//	<the island>
//	That `absent` is the whole feature in one line.
//
// rendered the two prose lines with a gap between them, referring to evidence
// the reader could not see. A green build is not evidence a human can read.
func TestAssertionsRenderEvidence(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    []string
		notWant []string
	}{
		{
			name: "shows renders which entities the world answered with",
			body: `shows{ type = "policy", world = "published", exactly = { "POL-1" }, absent = { "POL-2" } }`,
			want: []string{"✓ Verified", "the `published` world", "POL-1", "Not present, and not discoverable: POL-2"},
		},
		{
			name: "the default world is named in words, not left implicit",
			body: `shows{ type = "policy", exactly = { "POL-1", "POL-2" } }`,
			want: []string{"the default world", "POL-1, POL-2"},
		},
		{
			name: "hidden states that the row exists but is concealed",
			body: `hidden{ who = "pub", type = "policy", id = "POL-1" }`,
			want: []string{"cannot see", "role `reader`", "identical to one for an id"},
		},
		{
			name: "reads names the principal and their role",
			body: `reads{ who = "pub", type = "policy", id = "POL-1", face = "published" }`,
			want: []string{"can read", "`pub`", "role `reader`", "the `published` face"},
		},
		{
			name: "permits renders the role, not just the principal",
			body: `permits{ who = "ed", op = "update", type = "policy" }`,
			want: []string{"✓ Verified", "`ed`", "role `editor`", "may", "update"},
		},
		{
			name: "refuses renders the deciding rule",
			body: `refuses{ who = "pub", op = "update", type = "policy" }`,
			want: []string{"is **refused**", "Decided by"},
		},
		{
			name:    "emit=false suppresses the block entirely",
			body:    `shows{ type = "policy", world = "published", exactly = { "POL-1" }, emit = false }`,
			notWant: []string{"✓ Verified", "POL-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := "```rela\n" + readSeed + tc.body + "\n```\n"
			got := buildOut(t, src, Options{Meta: worldFixtureMeta(t), Policy: readFixturePolicy()})
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("rendered output missing %q:\n%s", w, got)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(got, w) {
					t.Errorf("emit=false still rendered %q:\n%s", w, got)
				}
			}
		})
	}
}

// TestEvidenceIsBlockquoted pins the rendered SHAPE, not just its words.
//
// The blockquote is what separates machine-verified evidence from the author's
// prose around it. A reader who cannot tell the two apart is reading assertions
// as claims, which is the state this work set out to leave behind.
func TestEvidenceIsBlockquoted(t *testing.T) {
	ev := evidence{
		claim:  "A is B.",
		header: []string{"Who", "Sees"},
		rows:   [][]string{{"pub", "POL-1"}},
		note:   "A note.",
	}
	got := ev.render()
	for line := range strings.SplitSeq(strings.TrimSpace(got), "\n") {
		if !strings.HasPrefix(line, ">") {
			t.Errorf("every evidence line must be blockquoted, got %q in:\n%s", line, got)
		}
	}
	for _, want := range []string{"✓ Verified", "A is B.", "| Who ", "| pub ", "A note."} {
		if !strings.Contains(got, want) {
			t.Errorf("render missing %q:\n%s", want, got)
		}
	}
}

// TestFieldBoolDefault pins the opt-out semantics: an ABSENT key must take the
// default, which is what makes `emit` default to rendering. fieldBool cannot
// express this — it reads absent and false identically.
func TestFieldBoolDefault(t *testing.T) {
	src := "```rela\n" + readSeed +
		`shows{ type = "policy", exactly = { "POL-1", "POL-2" } }` + "\n```\n"
	if got := buildOut(t, src, Options{Meta: worldFixtureMeta(t), Policy: readFixturePolicy()}); !strings.Contains(got, "✓ Verified") {
		t.Fatalf("an absent emit= must default to rendering, got:\n%s", got)
	}
}
