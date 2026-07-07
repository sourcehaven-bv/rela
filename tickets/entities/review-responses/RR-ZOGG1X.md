---
id: RR-ZOGG1X
type: review-response
title: Pattern-based HeaderCheck would leak a raw regex to the user in the detail
finding: 'For a pattern: HeaderCheck, GetMatchString() returns the raw regex (e.g. `## (Alternative|Alternatives)` — confirmed in metamodel/types.go). Showing ''Missing headers: ## (Alternative|Alternatives)'' in a tooltip is misleading — a user may try to add a literal header with that name. Fix: exclude pattern checks from the detail — MissingRequiredHeaders skips checks where IsPattern() is true; only report exact (Header) misses, which are literal actionable strings. Also sidesteps regex-in-tooltip escaping. Document as a scope boundary.'
severity: significant
resolution: Plan step 1 skips IsPattern() checks in MissingRequiredHeaders; documented as a scope boundary (only exact Header misses reported) with AC5 asserting exclusion.
status: addressed
---

**Finding:** For a `pattern:` HeaderCheck, `GetMatchString()` returns the raw
regex (e.g. `## (Alternative|Alternatives)` — confirmed in metamodel/types.go).
Showing "Missing headers: `## (Alternative|Alternatives)`" in a tooltip is
misleading — a user may try to add a literal header with that name.

**Resolution required in plan:** exclude pattern checks from the detail — only
report *exact* (`Header`) misses, which are literal, actionable strings.
`MissingRequiredHeaders` skips checks where `IsPattern()` is true. (95%
actionable case; also sidesteps regex-escaping concerns in the tooltip.)
Document this as a scope boundary in the plan.
