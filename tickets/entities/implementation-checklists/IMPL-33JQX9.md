---
id: IMPL-33JQX9
type: implementation-checklist
title: 'Implementation: Go 1.26.9 and x/net v0.60.0 for govulncheck findings'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: version bump and workflow config; no Go code)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: covered by the full CI run on the new toolchain)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests added)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: verified by `just govulncheck`; the scheduled workflow change can only run on GitHub)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: actionlint reports no new findings; the workflow path runs only on schedule)

**Verification Evidence:** `just govulncheck`: no actionable vulnerabilities.
`just ci` passes except TestChromeStyle_TargetsShippedClasses, which needs the
built SPA and fails only locally. actionlint: no new findings.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
