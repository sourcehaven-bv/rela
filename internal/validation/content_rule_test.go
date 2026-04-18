package validation

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestCheckContentRule(t *testing.T) {
	tests := []struct {
		name    string
		content string
		rule    *metamodel.ContentRule
		want    bool
	}{
		{
			name:    "nil rule passes",
			content: "# Title",
			rule:    nil,
			want:    true,
		},
		{
			name:    "empty rule passes",
			content: "# Title",
			rule:    &metamodel.ContentRule{},
			want:    true,
		},
		{
			name:    "required header present",
			content: "# Title\n## Context\nSome text",
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
				},
			},
			want: true,
		},
		{
			name:    "required header missing",
			content: "# Title\nSome text",
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
				},
			},
			want: false,
		},
		{
			name:    "multiple required headers all present",
			content: "# Title\n## Context\n## Decision\n## Alternatives",
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
					{Header: "## Decision"},
				},
			},
			want: true,
		},
		{
			name:    "multiple required headers one missing",
			content: "# Title\n## Context",
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{
					{Header: "## Context"},
					{Header: "## Decision"},
				},
			},
			want: false,
		},
		{
			name:    "pattern header present",
			content: "# Title\n## Alternatives",
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
			got := CheckContentRule(tt.content, tt.rule)
			if got != tt.want {
				t.Errorf("CheckContentRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckChecklistRule(t *testing.T) {
	tests := []struct {
		name  string
		items []markdown.ChecklistItem
		rule  *metamodel.ChecklistRule
		want  bool
	}{
		{
			name:  "nil rule passes",
			items: []markdown.ChecklistItem{{Checked: false}},
			rule:  nil,
			want:  true,
		},
		{
			name:  "empty items passes",
			items: []markdown.ChecklistItem{},
			rule:  &metamodel.ChecklistRule{AllChecked: true},
			want:  true,
		},
		{
			name: "all checked passes",
			items: []markdown.ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: true, Text: "Item 2"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true},
			want: true,
		},
		{
			name: "unchecked item fails",
			items: []markdown.ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: false, Text: "Item 2"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true},
			want: false,
		},
		{
			name: "skipped item passes with allow-skipped",
			items: []markdown.ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: false, Skipped: true, Text: "Skipped item"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true, AllowSkipped: true},
			want: true,
		},
		{
			name: "skipped item fails without allow-skipped",
			items: []markdown.ChecklistItem{
				{Checked: true, Text: "Item 1"},
				{Checked: false, Skipped: true, Text: "Skipped item"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true, AllowSkipped: false},
			want: false,
		},
		{
			name: "checked skipped item passes",
			items: []markdown.ChecklistItem{
				{Checked: true, Skipped: true, Text: "Item 1"},
			},
			rule: &metamodel.ChecklistRule{AllChecked: true},
			want: true,
		},
		{
			name: "no all-checked rule passes with unchecked",
			items: []markdown.ChecklistItem{
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
		name    string
		content string
		rule    *metamodel.ContentRule
		want    bool
	}{
		{
			name:    "all checked passes",
			content: "- [x] Task 1\n- [x] Task 2",
			rule: &metamodel.ContentRule{
				Checklist: &metamodel.ChecklistRule{AllChecked: true},
			},
			want: true,
		},
		{
			name:    "unchecked item fails",
			content: "- [x] Task 1\n- [ ] Task 2",
			rule: &metamodel.ContentRule{
				Checklist: &metamodel.ChecklistRule{AllChecked: true},
			},
			want: false,
		},
		{
			name:    "skipped item passes with allow-skipped",
			content: "- [x] Task 1\n- [x] ~~Task 2~~ (N/A)",
			rule: &metamodel.ContentRule{
				Checklist: &metamodel.ChecklistRule{AllChecked: true, AllowSkipped: true},
			},
			want: true,
		},
		{
			name:    "combined headers and checklist both pass",
			content: "## Summary\n- [x] Done\n## Details",
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{{Header: "## Summary"}},
				Checklist:       &metamodel.ChecklistRule{AllChecked: true},
			},
			want: true,
		},
		{
			name:    "combined headers pass checklist fails",
			content: "## Summary\n- [ ] Not done",
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{{Header: "## Summary"}},
				Checklist:       &metamodel.ChecklistRule{AllChecked: true},
			},
			want: false,
		},
		{
			name:    "combined headers fail checklist passes",
			content: "## Other\n- [x] Done",
			rule: &metamodel.ContentRule{
				RequiredHeaders: []metamodel.HeaderCheck{{Header: "## Summary"}},
				Checklist:       &metamodel.ChecklistRule{AllChecked: true},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckContentRule(tt.content, tt.rule)
			if got != tt.want {
				t.Errorf("CheckContentRule() = %v, want %v", got, tt.want)
			}
		})
	}
}
