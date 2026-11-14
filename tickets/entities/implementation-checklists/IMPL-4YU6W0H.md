---
id: IMPL-4YU6W0H
type: implementation-checklist
title: 'Implementation: Remove the jump bar from the entity detail page'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: removal only, no new code)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: removal only; existing EntityDetail suite covers the page)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error paths touched)

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

**Verification Evidence:** Built the SPA, ran `rela-server -project tickets` and
opened `/entity/ticket/TKT-UYC758H` (a view with 20 sections). The page shows
the Properties section directly under the header;
`document.querySelectorAll('.jump-bar,.jump-link')` returns 0. `grep -rn
"jump-bar\|jump-link\|scrollToSection" frontend/src e2e` returns nothing.
vue-tsc clean, ESLint 0 errors, EntityDetail Vitest suite 154/154. `grep -rni
"jump bar" docs docs-project` returns nothing.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: pure deletion)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no error paths)
- [x] No debug code left behind
