---
id: IMPL-A1WDJP0
type: implementation-checklist
title: 'Implementation: e2e: buildIfMissing passes -o after the package path'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the fixture is test code; the e2e suite has no unit tests for its fixtures)
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no test data)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no assertions added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no assertions added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no assertions added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no assertions added)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Extracted the new `buildIfMissing` body from
`fixtures.ts` into a node script and ran it with `CI` unset against absent
output paths. Both calls built: `./cmd/rela-server`, and `./cmd/rela-server`
with `-tags postgres` (`go version -m` shows `-tags=postgres`). The old order,
`go build ./cmd/rela-server -o <path>`, fails with `malformed import path "-o"`.
`npm run typecheck` and eslint on `fixtures.ts` pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
