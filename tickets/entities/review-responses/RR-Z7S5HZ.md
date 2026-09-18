---
id: RR-Z7S5HZ
type: review-response
title: 'golangci-lint: 32 issues on the changed packages would have failed CI'
finding: 'Code review. gocognit flagged checkRelationConstraint at complexity 35 (limit 30) — it had grown a third concern (the type filter) on top of two it already carried. Plus 13 misspell (British spellings in the new godocs: unrecognised, recognises, behaviour, honours), 17 modernize (the ptr/intp test helpers can be new(x) directly), 1 whitespace, and a perfsprint (fmt.Errorf with no verbs).'
severity: minor
resolution: Extracted the per-edge decision into Service.countsToward, which fixes gocognit and makes the three concerns it balances legible separately — the reviewer's suggested shape, and a genuine readability win rather than a linter workaround. Fixed all spellings, replaced the ptr/intp helpers with new(x), split the multi-line if, and switched to errors.New. golangci-lint now reports 0 issues across all six changed packages.
status: addressed
---
