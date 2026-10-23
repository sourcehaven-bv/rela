---
id: IMPL-KGZA20
type: implementation-checklist
title: 'Implementation: Triage osv-scanner findings from scheduled security scan (GH #1924)'
started: "2026-10-23"
completed: "2026-10-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; dependency override and scanner config only)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no new code; existing suites cover mermaid rendering)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no code paths changed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests added)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- osv-scanner 2.7.0 on `go.mod` and `frontend/package-lock.json`: GO-2026-5932
filtered by `osv-scanner.toml`; GHSA-238p-pmpm-9mq7 no longer reported. Newer
advisories (dompurify, storybook, vitest) remain; they have Dependabot PRs and
are out of scope.
- `npm ls katex`: mermaid@11.16.1 -> katex@0.18.10.
- katex smoke test: MathML render of `x^2+\sqrt{y}` works; with
`Object.prototype.trust = true` an `\href` is not rendered as a link.
- Frontend: 326 test files, 4708 tests pass; typecheck clean; lint 0 errors;
build succeeds.
- govulncheck: GO-2026-5932 at module level only (not called).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no code paths changed)
- [x] No debug code left behind
