package cli

import "testing"

func TestNormalizeHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty content",
			input: "",
			want:  "",
		},
		{
			name:  "no headers",
			input: "Just some text\nwithout headers",
			want:  "Just some text\nwithout headers",
		},
		{
			name:  "already at level 2",
			input: "## Overview\n### Details",
			want:  "## Overview\n### Details",
		},
		{
			name:  "already at level 3",
			input: "### Overview\n#### Details",
			want:  "### Overview\n#### Details",
		},
		{
			name:  "shift from level 1 to level 2",
			input: "# Overview\n## Details\n### Subsection",
			want:  "## Overview\n### Details\n#### Subsection",
		},
		{
			name:  "shift from level 1 with content",
			input: "# Title\n\nSome paragraph.\n\n## Section\n\nMore content.",
			want:  "## Title\n\nSome paragraph.\n\n### Section\n\nMore content.",
		},
		{
			name:  "preserve text after header",
			input: "# Header with text",
			want:  "## Header with text",
		},
		{
			name:  "skip headers in code blocks",
			input: "# Real Header\n```\n# Not a header\n## Also not\n```\n## Another real one",
			want:  "## Real Header\n```\n# Not a header\n## Also not\n```\n### Another real one",
		},
		{
			name:  "multiple code blocks",
			input: "# Title\n```bash\n# comment\n```\nText\n```\n# another\n```\n## Section",
			want:  "## Title\n```bash\n# comment\n```\nText\n```\n# another\n```\n### Section",
		},
		{
			name:  "cap at level 6",
			input: "# H1\n## H2\n### H3\n#### H4\n##### H5\n###### H6",
			want:  "## H1\n### H2\n#### H3\n##### H4\n###### H5\n###### H6",
		},
		{
			name:  "only level 1 headers",
			input: "# First\n# Second\n# Third",
			want:  "## First\n## Second\n## Third",
		},
		{
			name:  "mixed levels starting at 1",
			input: "Some intro\n\n# Main\n\n## Sub\n\n# Another Main",
			want:  "Some intro\n\n## Main\n\n### Sub\n\n## Another Main",
		},
		{
			name:  "trailing newline preserved",
			input: "# Header\n",
			want:  "## Header\n",
		},
		{
			name:  "hash in middle of line not affected",
			input: "# Header\nSome text with # in it\nMore # hashes",
			want:  "## Header\nSome text with # in it\nMore # hashes",
		},
		{
			name:  "hash without space not a header",
			input: "#hashtag\n# Real header",
			want:  "#hashtag\n## Real header",
		},
		{
			name:  "setext h1 converted to ATX",
			input: "Title\n=====\n\nContent",
			want:  "## Title\n\nContent",
		},
		{
			name:  "setext h2 not changed when min is h2",
			input: "Title\n-----\n\nContent",
			want:  "Title\n-----\n\nContent",
		},
		{
			name:  "mixed setext and ATX",
			input: "Main\n====\n\nSub\n---\n\n# ATX",
			want:  "## Main\n\n### Sub\n\n## ATX",
		},
		{
			name:  "setext with trailing spaces",
			input: "Title\n=====   \n\nContent",
			want:  "## Title\n\nContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeHeaders(tt.input)
			if got != tt.want {
				t.Errorf("normalizeHeaders() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestNormalizeCmd_DryRunFlagExists(t *testing.T) {
	flag := normalizeCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Error("normalize command should have --dry-run flag")
	}
}
