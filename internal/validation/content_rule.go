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

// MissingRequiredHeaders returns the match-strings of every *exact*
// required header the content is missing, for surfacing which headers a
// content-rule violation is about (e.g. in the data-entry analyze view).
//
// It is a detail-only helper, NOT the pass/fail authority — that stays
// CheckContentRule. Two deliberate narrowings versus CheckContentRule:
//
//   - Pattern header checks (IsPattern) are skipped: their match-string
//     is a raw regex, which is misleading to show a user as a header to
//     add. Only literal, actionable exact headers are reported.
//   - Checks with an empty match-string are skipped, mirroring the
//     trivial-pass semantics of MatchHeaderExact/MatchHeaderPattern (an
//     empty match satisfies trivially, so it is never "missing").
//
// Returns nil (not an empty slice) when nothing is missing, so callers
// can attach it conditionally and omitempty drops it on the wire.
func MissingRequiredHeaders(content string, rule *metamodel.ContentRule) []string {
	if rule == nil {
		return nil
	}

	headers := markdown.ExtractHeaders(content)
	var missing []string
	for _, hc := range rule.RequiredHeaders {
		if hc.IsPattern() || hc.GetMatchString() == "" {
			continue
		}
		if !matchHeaderCheck(headers, hc) {
			missing = append(missing, hc.GetMatchString())
		}
	}
	return missing
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
