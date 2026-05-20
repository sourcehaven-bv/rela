package fsstore

import (
	"errors"
	"testing"
)

// BUG-WN6D regression: the conflict-marker detector matched the
// substring `<<<<<<<` anywhere in a file, false-positiving on
// legitimate content like markdown codespans and quoted prose. The
// real semantic — git writes the marker at the start of a line — is
// now enforced by [hasLineAnchoredConflict].
func TestParseDocument_ConflictMarker_LineAnchored(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name:      "marker at column 0 is a conflict",
			content:   "---\nid: x\n---\n\n<<<<<<< HEAD\nfoo\n=======\nbar\n>>>>>>> branch\n",
			wantError: true,
		},
		{
			name:      "marker inside inline code span is NOT a conflict",
			content:   "---\nid: x\ntitle: how markers look\n---\n\nThe detector matches `<<<<<<<` at column 0.\n",
			wantError: false,
		},
		{
			name:      "marker mid-line in prose is NOT a conflict",
			content:   "---\nid: x\n---\n\nA literal <<<<<<< appears inline; not a conflict.\n",
			wantError: false,
		},
		{
			name:      "indented marker (two spaces) is NOT a conflict",
			content:   "---\nid: x\n---\n\n  <<<<<<< inside a block-quoted example\n",
			wantError: false,
		},
		{
			name:      "marker at column 0 of the very first line is a conflict",
			content:   "<<<<<<< HEAD\n---\nid: x\n---\n",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseDocument(tt.content)
			gotErr := errors.Is(err, errConflictedFile)
			if gotErr != tt.wantError {
				t.Errorf("parseDocument: errConflictedFile = %v, want %v (err=%v)",
					gotErr, tt.wantError, err)
			}
		})
	}
}

// Pin the helper predicate directly.
func TestHasLineAnchoredConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"empty", "", false},
		{"plain text", "no markers here\n", false},
		{"marker at start of file", "<<<<<<< HEAD\n", true},
		{"marker after newline", "intro\n<<<<<<< HEAD\n", true},
		{"marker after CRLF", "intro\r\n<<<<<<< HEAD\n", true},
		{"marker indented", "  <<<<<<< HEAD\n", false},
		{"marker after tab", "\t<<<<<<< HEAD\n", false},
		{"marker in codespan", "use `<<<<<<<` to find conflicts\n", false},
		{"marker mid-line", "prefix <<<<<<< suffix\n", false},
		{"multiple markers, first inline, second real", "inline <<<<<<< noise\n<<<<<<< HEAD\n", true},
		{"multiple markers, both inline", "first <<<<<<< noise\nthen `<<<<<<<` again\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasLineAnchoredConflict(tt.raw); got != tt.want {
				t.Errorf("hasLineAnchoredConflict(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
