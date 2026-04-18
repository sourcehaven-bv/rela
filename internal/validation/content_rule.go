package validation

import (
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// CheckContentRule validates markdown content against a metamodel-defined
// content rule. Returns true if the content satisfies the rule (or when
// rule is nil).
func CheckContentRule(content string, rule *metamodel.ContentRule) bool {
	if rule == nil {
		return true
	}

	headers := markdown.ExtractHeaders(content)
	for _, hc := range rule.RequiredHeaders {
		if !matchHeaderCheck(headers, hc) {
			return false
		}
	}

	if rule.Checklist != nil {
		items := markdown.ExtractChecklistItems(content)
		if !CheckChecklistRule(items, rule.Checklist) {
			return false
		}
	}

	return true
}

// CheckChecklistRule validates checklist items against a metamodel
// checklist rule. Returns true when items are acceptable (or when rule
// is nil, or when there are no items).
func CheckChecklistRule(items []markdown.ChecklistItem, rule *metamodel.ChecklistRule) bool {
	if rule == nil || len(items) == 0 {
		return true
	}

	if rule.AllChecked {
		for _, item := range items {
			if item.Checked {
				continue
			}
			if rule.AllowSkipped && item.Skipped {
				continue
			}
			return false
		}
	}

	return true
}

// matchHeaderCheck dispatches to the appropriate markdown primitive
// based on whether the check specifies a pattern or an exact header.
func matchHeaderCheck(headers []string, check metamodel.HeaderCheck) bool {
	if check.IsPattern() {
		return markdown.MatchHeaderPattern(headers, check.GetMatchString())
	}
	return markdown.MatchHeaderExact(headers, check.GetMatchString())
}
