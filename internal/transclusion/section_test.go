package transclusion

import (
	"testing"
)

func TestExtractSection(t *testing.T) {
	content := `# Title

Introduction paragraph.

## Overview

Overview content here.

### Details

Some details.

## Another Section

More content.
`

	tests := []struct {
		name    string
		heading string
		want    string
		found   bool
	}{
		{
			name:    "extract top-level section",
			heading: "Title",
			want:    "# Title\n\nIntroduction paragraph.\n\n## Overview\n\nOverview content here.\n\n### Details\n\nSome details.\n\n## Another Section\n\nMore content.",
			found:   true,
		},
		{
			name:    "extract nested section",
			heading: "Overview",
			want:    "## Overview\n\nOverview content here.\n\n### Details\n\nSome details.\n",
			found:   true,
		},
		{
			name:    "extract deeply nested section",
			heading: "Details",
			want:    "### Details\n\nSome details.\n",
			found:   true,
		},
		{
			name:    "extract section with sibling after",
			heading: "Another Section",
			want:    "## Another Section\n\nMore content.",
			found:   true,
		},
		{
			name:    "case insensitive",
			heading: "OVERVIEW",
			want:    "## Overview\n\nOverview content here.\n\n### Details\n\nSome details.\n",
			found:   true,
		},
		{
			name:    "not found",
			heading: "Nonexistent",
			want:    "",
			found:   false,
		},
		{
			name:    "empty heading returns all content",
			heading: "",
			want:    content,
			found:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := ExtractSection(content, tt.heading)
			if found != tt.found {
				t.Errorf("ExtractSection() found = %v, want %v", found, tt.found)
			}
			if got != tt.want {
				t.Errorf("ExtractSection() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestExtractSectionContent(t *testing.T) {
	content := `## Overview

This is the overview content.

More content here.

### Subsection

Subsection content.
`

	tests := []struct {
		name    string
		heading string
		want    string
		found   bool
	}{
		{
			name:    "extract content without heading",
			heading: "Overview",
			want:    "This is the overview content.\n\nMore content here.\n\n### Subsection\n\nSubsection content.",
			found:   true,
		},
		{
			name:    "extract subsection content",
			heading: "Subsection",
			want:    "Subsection content.",
			found:   true,
		},
		{
			name:    "not found",
			heading: "Nonexistent",
			want:    "",
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := ExtractSectionContent(content, tt.heading)
			if found != tt.found {
				t.Errorf("ExtractSectionContent() found = %v, want %v", found, tt.found)
			}
			if got != tt.want {
				t.Errorf("ExtractSectionContent() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestAdjustHeaderLevels(t *testing.T) {
	tests := []struct {
		name    string
		content string
		delta   int
		want    string
	}{
		{
			name:    "increase by 1",
			content: "# Title\n\nText\n\n## Subsection",
			delta:   1,
			want:    "## Title\n\nText\n\n### Subsection",
		},
		{
			name:    "decrease by 1",
			content: "## Title\n\nText\n\n### Subsection",
			delta:   -1,
			want:    "# Title\n\nText\n\n## Subsection",
		},
		{
			name:    "no change",
			content: "# Title\n\nText",
			delta:   0,
			want:    "# Title\n\nText",
		},
		{
			name:    "clamp at level 6",
			content: "###### Level 6",
			delta:   1,
			want:    "###### Level 6",
		},
		{
			name:    "clamp at level 1",
			content: "# Level 1",
			delta:   -1,
			want:    "# Level 1",
		},
		{
			name:    "increase by 2",
			content: "# Title\n## Sub",
			delta:   2,
			want:    "### Title\n#### Sub",
		},
		{
			name:    "no headers",
			content: "Just text\nMore text",
			delta:   1,
			want:    "Just text\nMore text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AdjustHeaderLevels(tt.content, tt.delta)
			if got != tt.want {
				t.Errorf("AdjustHeaderLevels() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestGetHeaderLevel(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"# Title", 1},
		{"## Title", 2},
		{"### Title", 3},
		{"#### Title", 4},
		{"##### Title", 5},
		{"###### Title", 6},
		{"No header", 0},
		{"", 0},
		{"# Title\n## Sub", 1},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			got := GetHeaderLevel(tt.content)
			if got != tt.want {
				t.Errorf("GetHeaderLevel(%q) = %d, want %d", tt.content, got, tt.want)
			}
		})
	}
}

func TestStripFirstHeader(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "strip header",
			content: "# Title\n\nContent here",
			want:    "Content here",
		},
		{
			name:    "no header",
			content: "No header\nJust text",
			want:    "No header\nJust text",
		},
		{
			name:    "header only",
			content: "# Title",
			want:    "",
		},
		{
			name:    "multiple headers",
			content: "# Title\n## Sub\nContent",
			want:    "## Sub\nContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripFirstHeader(tt.content)
			if got != tt.want {
				t.Errorf("StripFirstHeader() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
