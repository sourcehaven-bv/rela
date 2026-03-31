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

func TestCheckChecklistRule(t *testing.T) {
	tests := []struct {
		name  string
		items []ChecklistItem
		rule  *metamodel.ChecklistRule
		want  bool
	}{
		{
			name:  "nil rule passes",
			items: []ChecklistItem{{Checked: false}},
			rule:  nil,
			want:  true,
		},
		{
			name:  "empty items passes",
			items: []ChecklistItem{},
			rule:  &metamodel.ChecklistRule{AllChecked: true},
			want:  true,
		},
		{
			name: "all checked passes",
			items: []ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: true, Text: "Item 2"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true},
			want: true,
		},
		{
			name: "unchecked item fails",
			items: []ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: false, Text: "Item 2"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true},
			want: false,
		},
		{
			name: "skipped item passes with allow-skipped",
			items: []ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: false, Skipped: true, Text: "Skipped item"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true, AllowSkipped: true},
			want: true,
		},
		{
			name: "skipped item fails without allow-skipped",
			items: []ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: false, Skipped: true, Text: "Skipped item"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true, AllowSkipped: false},
			want: false,
		},
		{
			name: "checked skipped item passes",
			items: []ChecklistItem{
				{Checked: true, Skipped: true, Text: "Item 1"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true},
			want: true,
		},
		{
			name: "no all-checked rule passes with unchecked",
			items: []ChecklistItem{
				{Checked: false, Text: "Unchecked"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: false},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckChecklistRule(tt.items, tt.rule)
			if got != tt.want {
				t.Errorf("CheckChecklistRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckContentRuleWithChecklist(t *testing.T) {
	tests := []struct {
		name   string
		entity *model.Entity
		rule   *metamodel.ContentRule
		want   bool
	}{
		{
			name:   "all checked passes",
			entity: &model.Entity{Content: "- [x] Task 1\n- [x] Task 2"},
			rule: &metamodel.ContentRule{
				Checklist: &metamodel.ChecklistRule{AllChecked: true},
			},
			want: true,
		},
		{
			name:   "unchecked item fails",
			entity: &model.Entity{Content: "- [x] Task 1\n- [ ] Task 2"},
			rule: &metamodel.ContentRule{
				Checklist: &metamodel.ChecklistRule{AllChecked: true},
			},
			want: false,
		},
		{
			name:   "skipped item passes with allow-skipped",
			entity: &model.Entity{Content: "- [x] Task 1\n- [x] ~~Task 2~~ (N/A)"},
			rule: &metamodel.ContentRule{
				Checklist: &metamodel.ChecklistRule{AllChecked: true, AllowSkipped: true},
			},
			want: true,
		},
		{
			name:   "combined headers and checklist both pass",
			entity: &model.Entity{Content: "## Summary\n- [x] Done\n## Details"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{{Header: "## Summary"}},
				Checklist:       &metamodel.ChecklistRule{AllChecked: true},
			},
			want: true,
		},
		{
			name:   "combined headers pass checklist fails",
			entity: &model.Entity{Content: "## Summary\n- [ ] Not done"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{{Header: "## Summary"}},
				Checklist:       &metamodel.ChecklistRule{AllChecked: true},
			},
			want: false,
		},
		{
			name:   "combined headers fail checklist passes",
			entity: &model.Entity{Content: "## Other\n- [x] Done"},
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{{Header: "## Summary"}},
				Checklist:       &metamodel.ChecklistRule{AllChecked: true},
			},
			want: false,
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
