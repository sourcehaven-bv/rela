---
id: IMPL-BP2I6N
type: implementation-checklist
title: 'Implementation: govulncheck auto-update picks wrong Go minor and duplicates issues'
started: "2026-10-25"
completed: "2026-10-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the full flow is a scheduled workflow; the fix picker is tested in CI and the dedup query was run against the live repo)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: shell test compares printed lines)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: shell test compares printed lines)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `just govulncheck` on go1.26.10: "no actionable vulnerabilities found"
(GO-2026-6809 cleared).
- New `govulncheck-fixable.sh` on the real go1.26.9 JSON prints only
`stdlib 1.26.10`; the old script printed 1.26.10 and 1.27.3.
- `scripts/govulncheck-fixable-test.sh`: 7/7 pass; 3/7 fail against the old
script.
- Title-only dedup query against the repo returns #1986 #1970 #1962 #1953
  #1945, so the next run would comment on #1986.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
