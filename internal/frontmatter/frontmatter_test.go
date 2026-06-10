package frontmatter

import (
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantFM   string
		wantBody string
	}{
		{
			name:     "frontmatter and body",
			content:  "---\nid: REQ-001\ntype: requirement\n---\n\n# Heading\n\nContent here.\n",
			wantFM:   "id: REQ-001\ntype: requirement",
			wantBody: "# Heading\n\nContent here.",
		},
		{
			name:     "no frontmatter is all body",
			content:  "# Just a heading\n\nSome content.\n",
			wantFM:   "",
			wantBody: "# Just a heading\n\nSome content.",
		},
		{
			name:     "empty content",
			content:  "",
			wantFM:   "",
			wantBody: "",
		},
		{
			name:     "only frontmatter",
			content:  "---\nid: REQ-001\n---\n",
			wantFM:   "id: REQ-001",
			wantBody: "",
		},
		{
			name:     "CRLF line endings are normalized",
			content:  "---\r\nid: REQ-001\r\n---\r\n\r\nBody line\r\n",
			wantFM:   "id: REQ-001",
			wantBody: "Body line",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm, body := Split(tc.content)
			if fm != tc.wantFM {
				t.Errorf("frontmatter = %q, want %q", fm, tc.wantFM)
			}
			if body != tc.wantBody {
				t.Errorf("body = %q, want %q", body, tc.wantBody)
			}
		})
	}
}

// TestSplit_LongLineDoesNotFail is the BUG-* regression: a single line
// far larger than bufio.MaxScanTokenSize (64 KB) must split cleanly. The
// previous bufio.Scanner implementation returned bufio.ErrTooLong here,
// making such a file writable but unreadable.
func TestSplit_LongLineExceeds64KB(t *testing.T) {
	const bigLineSize = 256 * 1024 // 4x the scanner's 64 KB cap
	bigValue := strings.Repeat("x", bigLineSize)
	content := "---\nid: REQ-001\n---\n\n" + bigValue + "\n"

	fm, body := Split(content)
	if fm != "id: REQ-001" {
		t.Errorf("frontmatter = %q, want id: REQ-001", fm)
	}
	if body != bigValue {
		t.Errorf("body length = %d, want %d (long line was truncated or dropped)", len(body), bigLineSize)
	}
}

// TestSplit_LongLineInFrontmatter covers the same cap on the
// frontmatter side (e.g. a long base64 property value).
func TestSplit_LongLineInFrontmatter(t *testing.T) {
	const bigLineSize = 128 * 1024
	bigValue := strings.Repeat("y", bigLineSize)
	content := "---\nimage: " + bigValue + "\n---\nBody\n"

	fm, body := Split(content)
	if !strings.HasPrefix(fm, "image: ") || len(fm) < bigLineSize {
		t.Errorf("frontmatter length = %d, want >= %d", len(fm), bigLineSize)
	}
	if body != "Body" {
		t.Errorf("body = %q, want Body", body)
	}
}
