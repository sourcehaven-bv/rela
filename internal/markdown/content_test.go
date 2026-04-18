package markdown

import "testing"

func TestExtractHeaders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "no headers",
			content: "Just some text\nwithout headers",
			want:    nil,
		},
		{
			name:    "single header",
			content: "# Title\nSome content",
			want:    []string{"# Title"},
		},
		{
			name:    "multiple levels",
			content: "# H1\n## H2\n### H3\ntext",
			want:    []string{"# H1", "## H2", "### H3"},
		},
		{
			name:    "headers with whitespace",
			content: "  ## Context  \ntext\n   ### Details   ",
			want:    []string{"## Context", "### Details"},
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "ignores headers in code blocks",
			content: "# Real Header\n```\n## Not a header\n# Also not a header\n```\n## Another Real Header",
			want:    []string{"# Real Header", "## Another Real Header"},
		},
		{
			name:    "ignores headers in indented code blocks",
			content: "# Title\n\n    ## Indented code\n    # More code\n\n## Actual Header",
			want:    []string{"# Title", "## Actual Header"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractHeaders(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractHeaders() got %d headers, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("header[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMatchHeaderExact(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		exact   string
		want    bool
	}{
		{
			name:    "match found",
			headers: []string{"# Title", "## Context", "## Decision"},
			exact:   "## Context",
			want:    true,
		},
		{
			name:    "not found",
			headers: []string{"# Title", "## Decision"},
			exact:   "## Context",
			want:    false,
		},
		{
			name:    "empty exact matches trivially",
			headers: []string{"# Title"},
			exact:   "",
			want:    true,
		},
		{
			name:    "empty headers no match",
			headers: []string{},
			exact:   "## Context",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchHeaderExact(tt.headers, tt.exact)
			if got != tt.want {
				t.Errorf("MatchHeaderExact() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchHeaderPattern(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		pattern string
		want    bool
	}{
		{
			name:    "match found",
			headers: []string{"# Title", "## Alternatives"},
			pattern: "## (Alternative|Alternatives)",
			want:    true,
		},
		{
			name:    "not found",
			headers: []string{"# Title", "## Other"},
			pattern: "## (Alternative|Alternatives)",
			want:    false,
		},
		{
			name:    "empty pattern matches trivially",
			headers: []string{"# Title"},
			pattern: "",
			want:    true,
		},
		{
			name:    "invalid regex returns false",
			headers: []string{"## Test"},
			pattern: "[invalid",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchHeaderPattern(tt.headers, tt.pattern)
			if got != tt.want {
				t.Errorf("MatchHeaderPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractChecklistItems(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []ChecklistItem
	}{
		{
			name:    "empty content",
			content: "",
			want:    nil,
		},
		{
			name:    "no checklists",
			content: "Just some text\n- Regular list item\n- Another item",
			want:    nil,
		},
		{
			name:    "single unchecked item",
			content: "- [ ] Task to do",
			want: []ChecklistItem{
				{Checked: false, Skipped: false, Text: "Task to do"},
			},
		},
		{
			name:    "single checked item",
			content: "- [x] Completed task",
			want: []ChecklistItem{
				{Checked: true, Skipped: false, Text: "Completed task"},
			},
		},
		{
			name:    "uppercase X",
			content: "- [X] Also checked",
			want: []ChecklistItem{
				{Checked: true, Skipped: false, Text: "Also checked"},
			},
		},
		{
			name:    "multiple items",
			content: "- [x] Done\n- [ ] Not done\n- [x] Also done",
			want: []ChecklistItem{
				{Checked: true, Skipped: false, Text: "Done"},
				{Checked: false, Skipped: false, Text: "Not done"},
				{Checked: true, Skipped: false, Text: "Also done"},
			},
		},
		{
			name:    "strikethrough item",
			content: "- [x] ~~Skipped task~~ (N/A: reason)",
			want: []ChecklistItem{
				{Checked: true, Skipped: true, Text: "Skipped task (N/A: reason)"},
			},
		},
		{
			name:    "unchecked strikethrough",
			content: "- [ ] ~~This was skipped~~",
			want: []ChecklistItem{
				{Checked: false, Skipped: true, Text: "This was skipped"},
			},
		},
		{
			name:    "mixed checked unchecked and skipped",
			content: "- [x] Done task\n- [ ] Pending\n- [x] ~~Skipped~~ (N/A)",
			want: []ChecklistItem{
				{Checked: true, Skipped: false, Text: "Done task"},
				{Checked: false, Skipped: false, Text: "Pending"},
				{Checked: true, Skipped: true, Text: "Skipped (N/A)"},
			},
		},
		{
			name:    "nested checklist",
			content: "- [x] Parent\n  - [ ] Child 1\n  - [x] Child 2",
			want: []ChecklistItem{
				{Checked: true, Skipped: false, Text: "Parent"},
				{Checked: false, Skipped: false, Text: "Child 1"},
				{Checked: true, Skipped: false, Text: "Child 2"},
			},
		},
		{
			name:    "checkbox in code block ignored",
			content: "- [x] Real item\n```\n- [ ] Not a real item\n```",
			want: []ChecklistItem{
				{Checked: true, Skipped: false, Text: "Real item"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractChecklistItems(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractChecklistItems() got %d items, want %d", len(got), len(tt.want))
				for i, item := range got {
					t.Logf("  got[%d]: checked=%v, skipped=%v, text=%q", i, item.Checked, item.Skipped, item.Text)
				}
				return
			}
			for i := range got {
				if got[i].Checked != tt.want[i].Checked {
					t.Errorf("item[%d].Checked = %v, want %v", i, got[i].Checked, tt.want[i].Checked)
				}
				if got[i].Skipped != tt.want[i].Skipped {
					t.Errorf("item[%d].Skipped = %v, want %v", i, got[i].Skipped, tt.want[i].Skipped)
				}
				if got[i].Text != tt.want[i].Text {
					t.Errorf("item[%d].Text = %q, want %q", i, got[i].Text, tt.want[i].Text)
				}
			}
		})
	}
}
