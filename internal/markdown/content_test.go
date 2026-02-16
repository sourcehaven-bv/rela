package markdown

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

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

func TestMatchHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		check   metamodel.HeaderCheck
		want    bool
	}{
		{
			name:    "exact match found",
			headers: []string{"# Title", "## Context", "## Decision"},
			check:   metamodel.HeaderCheck{Header: "## Context"},
			want:    true,
		},
		{
			name:    "exact match not found",
			headers: []string{"# Title", "## Decision"},
			check:   metamodel.HeaderCheck{Header: "## Context"},
			want:    false,
		},
		{
			name:    "pattern match found",
			headers: []string{"# Title", "## Alternatives"},
			check:   metamodel.HeaderCheck{Pattern: "## (Alternative|Alternatives)"},
			want:    true,
		},
		{
			name:    "pattern match not found",
			headers: []string{"# Title", "## Other"},
			check:   metamodel.HeaderCheck{Pattern: "## (Alternative|Alternatives)"},
			want:    false,
		},
		{
			name:    "empty check matches",
			headers: []string{"# Title"},
			check:   metamodel.HeaderCheck{},
			want:    true,
		},
		{
			name:    "empty headers no match",
			headers: []string{},
			check:   metamodel.HeaderCheck{Header: "## Context"},
			want:    false,
		},
		{
			name:    "invalid regex returns false",
			headers: []string{"## Test"},
			check:   metamodel.HeaderCheck{Pattern: "[invalid"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchHeader(tt.headers, tt.check)
			if got != tt.want {
				t.Errorf("MatchHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckContentRule(t *testing.T) {
	tests := []struct {
		name   string
		entity *model.Entity
		rule   *metamodel.ContentRule
		want   bool
	}{
		{
			name:   "nil rule passes",
			entity: &model.Entity{Content: "# Title"},
			rule:   nil,
			want:   true,
		},
		{
			name:   "empty rule passes",
			entity: &model.Entity{Content: "# Title"},
			rule:   &metamodel.ContentRule{},
			want:   true,
		},
		{
			name:   "required header present",
			entity: &model.Entity{Content: "# Title\n## Context\nSome text"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
				},
			},
			want: true,
		},
		{
			name:   "required header missing",
			entity: &model.Entity{Content: "# Title\nSome text"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
				},
			},
			want: false,
		},
		{
			name:   "multiple required headers all present",
			entity: &model.Entity{Content: "# Title\n## Context\n## Decision\n## Alternatives"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
					{Header: "## Decision"},
				},
			},
			want: true,
		},
		{
			name:   "multiple required headers one missing",
			entity: &model.Entity{Content: "# Title\n## Context"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
					{Header: "## Decision"},
				},
			},
			want: false,
		},
		{
			name:   "pattern header present",
			entity: &model.Entity{Content: "# Title\n## Alternatives"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Pattern: "## (Alternative|Alternatives)"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckContentRule(tt.entity, tt.rule)
			if got != tt.want {
				t.Errorf("CheckContentRule() = %v, want %v", got, tt.want)
			}
		})
	}
}
