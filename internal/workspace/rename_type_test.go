package workspace

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestReplaceYAMLType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		newType  string
		want     string
		replaced bool
	}{
		{
			name: "replaces top-level type",
			input: `---
id: REQ-1
type: requirement
title: Something
---

# Body
`,
			newType: "feature",
			want: `---
id: REQ-1
type: feature
title: Something
---

# Body
`,
			replaced: true,
		},
		{
			name: "no frontmatter returns unchanged",
			input: `# Just body
type: not-yaml
`,
			newType:  "feature",
			replaced: false,
		},
		{
			name: "no type field returns unchanged",
			input: `---
id: REQ-1
title: Something
---
`,
			newType:  "feature",
			replaced: false,
		},
		{
			name: "ignores indented type: inside nested maps",
			input: `---
id: REQ-1
meta:
  type: should-not-match
type: requirement
---
`,
			newType: "feature",
			want: `---
id: REQ-1
meta:
  type: should-not-match
type: feature
---
`,
			replaced: true,
		},
		{
			name: "unterminated frontmatter stops searching at closing ---",
			input: `---
id: REQ-1
---
body has type: foo
`,
			newType:  "feature",
			replaced: false,
		},
		{
			name:     "empty input",
			input:    "",
			newType:  "feature",
			replaced: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := replaceYAMLType(tt.input, tt.newType)
			if ok != tt.replaced {
				t.Errorf("replaced = %v, want %v", ok, tt.replaced)
			}
			if tt.replaced && got != tt.want {
				t.Errorf("output mismatch\n got:  %q\n want: %q", got, tt.want)
			}
			if !tt.replaced && got != tt.input {
				t.Errorf("unchanged expected but got different:\n got:  %q\n want: %q", got, tt.input)
			}
		})
	}
}

func TestRewriteEntityTypeInFile(t *testing.T) {
	fs := storage.NewMemFS()
	path := "/entities/REQ-1.md"
	content := `---
id: REQ-1
type: requirement
title: hi
---
body
`
	if err := fs.MkdirAll("/entities", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := rewriteEntityTypeInFile(fs, path, "feature"); err != nil {
		t.Fatalf("error: %v", err)
	}
	got, err := fs.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "type: feature") {
		t.Errorf("file content after rewrite:\n%s", got)
	}
}

func TestRewriteEntityTypeInFile_NoTypeLeavesUntouched(t *testing.T) {
	fs := storage.NewMemFS()
	path := "/entities/REQ-1.md"
	original := `---
id: REQ-1
title: no type field
---
body
`
	_ = fs.MkdirAll("/entities", 0o755)
	_ = fs.WriteFile(path, []byte(original), 0o644)

	if err := rewriteEntityTypeInFile(fs, path, "feature"); err != nil {
		t.Fatal(err)
	}

	got, _ := fs.ReadFile(path)
	if string(got) != original {
		t.Error("file should be unchanged when no type field")
	}
}

func TestRewriteEntityTypeInDir(t *testing.T) {
	fs := storage.NewMemFS()
	_ = fs.MkdirAll("/entities/reqs", 0o755)
	_ = fs.WriteFile("/entities/reqs/REQ-1.md", []byte(`---
id: REQ-1
type: requirement
---
`), 0o644)
	_ = fs.WriteFile("/entities/reqs/REQ-2.md", []byte(`---
id: REQ-2
type: requirement
---
`), 0o644)
	// Non-markdown file should be skipped.
	_ = fs.WriteFile("/entities/reqs/README.txt", []byte("hi"), 0o644)

	count, err := rewriteEntityTypeInDir(fs, "/entities/reqs", "feature")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	got1, _ := fs.ReadFile("/entities/reqs/REQ-1.md")
	if !strings.Contains(string(got1), "type: feature") {
		t.Errorf("REQ-1 not updated: %s", got1)
	}
}

func TestRewriteEntityTypeInDir_MissingDir(t *testing.T) {
	fs := storage.NewMemFS()
	count, err := rewriteEntityTypeInDir(fs, "/does/not/exist", "feature")
	if err != nil {
		t.Errorf("expected no error for missing dir, got %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}
