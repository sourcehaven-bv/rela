package markdown

import (
	"reflect"
	"strings"
	"testing"
)

// conflictExampleBody is a markdown body that documents git's conflict
// markers, the case BUG-GYM7ZBS found reading as an unresolved merge. A lone
// marker line is text, and a complete example indents its marker lines in
// its code fence.
const conflictExampleBody = "# Resolving a merge\n" +
	"\n" +
	"A line that starts with\n" +
	"<<<<<<< opens the block, but on its own it is just text.\n" +
	"\n" +
	"Git writes:\n" +
	"\n" +
	"```text\n" +
	"  <<<<<<< HEAD\n" +
	"  status: draft\n" +
	"  =======\n" +
	"  status: approved\n" +
	"  >>>>>>> feature\n" +
	"```\n" +
	"\n" +
	"```text\n" +
	"<<<<<<< HEAD\n" +
	"=======\n" +
	"```\n"

// TestConflictBlocks pins what counts as a git conflict block: start,
// separator and end markers at column 0, in that order, as git writes them.
func TestConflictBlocks(t *testing.T) {
	t.Parallel()
	const block = "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> feature\n"
	tests := []struct {
		name    string
		content string
		want    []ConflictBlock
	}{
		{"empty", "", nil},
		{"complete block", block, []ConflictBlock{{1, 3, 5, "HEAD", "feature"}}},
		{"unlabelled markers", "<<<<<<<\na\n=======\nb\n>>>>>>>\n", []ConflictBlock{{1, 3, 5, "", ""}}},
		{
			"diff3 block",
			"<<<<<<< HEAD\na\n||||||| base\no\n=======\nb\n>>>>>>> feature\n",
			[]ConflictBlock{{1, 5, 7, "HEAD", "feature"}},
		},
		{"lone start marker", "intro\n<<<<<<< HEAD\nmore\n", nil},
		{"start and separator only", "<<<<<<< HEAD\na\n=======\nb\n", nil},
		{"start and end only", "<<<<<<< HEAD\na\n>>>>>>> feature\n", nil},
		{"markers out of order", ">>>>>>> feature\n=======\n<<<<<<< HEAD\n", nil},
		{"end before separator", "<<<<<<< HEAD\n>>>>>>> x\n=======\n", nil},
		{"indented markers", "  <<<<<<< HEAD\n  =======\n  >>>>>>> feature\n", nil},
		{"longer marker runs", "<<<<<<<< HEAD\n========\n>>>>>>>> feature\n", nil},
		{"setext underline is no separator", "<<<<<<< x\nTitle\n==========\n>>>>>>> y\n", nil},
		{"marker glued to text", "<<<<<<<HEAD\n=======\n>>>>>>>feature\n", nil},
		{"documenting body", conflictExampleBody, nil},
		{
			"block after a documenting body",
			conflictExampleBody + "\n" + block,
			[]ConflictBlock{{21, 23, 25, "HEAD", "feature"}},
		},
		{
			// A new start marker begins the block again.
			"second start marker restarts the block",
			"<<<<<<< stale\n<<<<<<< HEAD\na\n=======\nb\n>>>>>>> feature\n",
			[]ConflictBlock{{2, 4, 6, "HEAD", "feature"}},
		},
		{
			// Git moves a fence line both sides share above the start marker
			// and the shared closing fence below the end marker. The merge is
			// still found, and so is the one after it.
			"merge inside a code block",
			"```go\n" + block + "```\n\n" + block,
			[]ConflictBlock{{2, 4, 6, "HEAD", "feature"}, {9, 11, 13, "HEAD", "feature"}},
		},
		{
			"fence opened inside a side",
			"<<<<<<< HEAD\n```go\n=======\nx\n>>>>>>> feature\n" + block,
			[]ConflictBlock{{1, 3, 5, "HEAD", "feature"}, {6, 8, 10, "HEAD", "feature"}},
		},
		{
			// A fence line in a frontmatter block scalar hides nothing.
			"fence in a block scalar before a frontmatter conflict",
			"---\ndescription: |\n  ```go\nid: x\n" + block + "---\n",
			[]ConflictBlock{{5, 7, 9, "HEAD", "feature"}},
		},
		{"crlf block", strings.ReplaceAll(block, "\n", "\r\n"), []ConflictBlock{{1, 3, 5, "HEAD", "feature"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ConflictBlocks(tc.content); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ConflictBlocks = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestParseDocument_BodyDocumentingConflictMarkers reads back a document
// whose body documents conflict markers (BUG-GYM7ZBS).
func TestParseDocument_BodyDocumentingConflictMarkers(t *testing.T) {
	t.Parallel()
	formatted, err := FormatDocument(map[string]any{"id": "GUIDE-1"}, conflictExampleBody)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	doc, err := ParseDocument(formatted)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if !strings.Contains(doc.Content, "<<<<<<< opens the block") || !strings.Contains(doc.Content, ">>>>>>> feature") {
		t.Errorf("body lost the documented markers:\n%s", doc.Content)
	}
}
