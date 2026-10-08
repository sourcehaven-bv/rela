package comments_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/comments"
)

// segmentCase is one golden entry: a body, a range in it, the segments the
// server computes for that range, and the text each resulting mark must show
// once the SPA renders the body.
//
// The Go side cannot run the SPA's renderer, and the SPA cannot run goldmark,
// so the two agree through this file:
// frontend/src/utils/commentHighlight.golden.test.ts splices the segments in,
// renders with marked, and checks the marks. A case where goldmark and marked
// disagree on block structure fails there.
type segmentCase struct {
	Name     string          `json:"name"`
	Body     string          `json:"body"`
	Start    int             `json:"start"`
	End      int             `json:"end"`
	Segments []comments.Span `json:"segments"`
	Marks    []string        `json:"marks"`
}

// segmentFixtures are the shapes where goldmark and marked could plausibly
// disagree, or where a mark edge could break the markup it sits in. From and
// To pick the range: the first occurrence of From through the end of the first
// occurrence of To after it.
var segmentFixtures = []struct {
	name, body, from, to string
	marks                []string
}{
	{"heading into body", "## Setup\n\nInstall the tools first.\n", "Setup", "the tools",
		[]string{"Setup", "Install the tools"}},
	{"atx closing hashes", "## Setup ##\n\nInstall the tools first.\n", "Setup", "Install",
		[]string{"Setup", "Install"}},
	{"setext heading", "Setup\n=====\n\nInstall the tools first.\n", "Setup", "Install",
		[]string{"Setup", "Install"}},
	{"link reference definition between", "Intro text here.\n\n[ref]: https://example.com\n\nSee [the ref][ref] now.\n",
		"text here", "See", []string{"text here.", "See"}},
	{"html block skipped", "Intro text here.\n\n<div>\nraw block\n</div>\n\nAfter the block.\n",
		"text here", "After", []string{"text here.", "After"}},
	{"html comment interrupts paragraph", "Intro text here.\n<!-- note -->\nAfter the note.\n",
		"text here", "After", []string{"text here.", "After"}},
	{"nested list", "- outer item\n    - inner item\n- last item\n", "outer", "last",
		[]string{"outer item", "inner item", "last"}},
	{"blockquote continuation", "Intro.\n\n> quote line\n> second line\n", "Intro", "second",
		[]string{"Intro.", "quote line\nsecond"}},
	{"table right after paragraph", "Intro text.\n| a | b |\n|---|---|\n| one | two |\n", "text", "two",
		[]string{"text.", "a", "b", "one", "two"}},
	{"fenced code skipped", "Intro text.\n\n```\ncode line\n```\n\nAfter text.\n", "text", "After",
		[]string{"text.", "After"}},
	{"inline code included whole", "Intro text.\n\nRun `make` now.\n", "text", "now",
		[]string{"text.", "Run make now"}},
	{"start inside inline code", "Run `make test` now.\n\nNext paragraph.\n", "test", "Next",
		[]string{"make test now.", "Next"}},
	{"end inside inline code", "Intro text.\n\nRun `make test` now.\n", "text", "make",
		[]string{"text.", "Run make test"}},
	{"double-backtick code span", "Intro text.\n\nUse `` a`b `` here.\n", "text", "here",
		[]string{"text.", "Use a`b here"}},
	{"trailing backslash", "Path is C:\\dir\\\n\nNext paragraph.\n", "Path", "Next",
		[]string{"Path is C:\\dir", "Next"}},
	{"task item starting with emphasis", "Intro text.\n\n- [ ] **bold** rest\n- [x] [link](https://example.com) more\n",
		"text", "more", []string{"text.", "bold rest", "link more"}},
	{"start inside emphasis", "Read the **important** text.\n\nNext paragraph.\n", "portant", "Next",
		[]string{"important text.", "Next"}},
	{"start inside link", "See [the docs](https://example.com) here.\n\nNext paragraph.\n", "docs", "Next",
		[]string{"the docs here.", "Next"}},
	{"end inside emphasis", "Intro text.\n\nRead the **important** text.\n", "text", "impor",
		[]string{"text.", "Read the impor"}},
}

func TestSegmentsGolden(t *testing.T) {
	out := make([]segmentCase, 0, len(segmentFixtures))
	for _, f := range segmentFixtures {
		start := strings.Index(f.body, f.from)
		require.GreaterOrEqual(t, start, 0, f.name)
		rel := strings.Index(f.body[start:], f.to)
		require.GreaterOrEqual(t, rel, 0, f.name)
		end := start + rel + len(f.to)

		a, err := comments.NewTextAnchor(f.body, start, end)
		require.NoError(t, err, f.name)
		m := comments.NewBody(f.body).ResolveText(a)
		require.False(t, m.Detached, f.name)
		require.Equal(t, start, m.Start, f.name)
		require.Equal(t, end, m.End, f.name)

		out = append(out, segmentCase{
			Name: f.name, Body: f.body, Start: start, End: end,
			Segments: m.Segments, Marks: f.marks,
		})
	}

	const goldenPath = "testdata/segments_golden.json"
	encoded, err := json.MarshalIndent(out, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')

	if os.Getenv("UPDATE_GOLDENS") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(goldenPath, encoded, 0o644))
		return
	}
	existing, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "missing %s; run with UPDATE_GOLDENS=1 to create it", goldenPath)
	require.Equal(t, string(existing), string(encoded),
		"segments drifted from the golden file; if intended, regenerate with UPDATE_GOLDENS=1 and rerun the frontend test")
}
