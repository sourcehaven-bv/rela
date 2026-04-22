---
id: IMPL-DATER
type: implementation-checklist
title: 'Implementation: Consolidate frontend/e2e into /e2e with fixes and CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: this is test-infrastructure work; the new code *is* the tests)
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- **AC1** (`/frontend/e2e/` deleted, playwright scripts/devDep removed):
`ls frontend/e2e` → no such path; `grep -E "playwright|test:e2e"
frontend/package.json` → no matches.
- **AC2** (`just e2e` passes, all tests pass locally):
`npm test` in `/e2e/` — 174 passed, 5 skipped (intentional: 3
document-live-update, 1 template selector, 1 checkbox toggle). Ran 5 consecutive
times with `--retries=0`; one flake on `list › Keyboard Navigation › can
navigate rows with keyboard` (race between focus and ArrowDown); fixed by
waiting for first row visibility instead of clicking the table.
- **AC3** (CI `e2e` job gates `build`): `.github/workflows/ci.yml` `build.needs` still lists `e2e`. Added `npm run lint` step to the `e2e` job so POP violations fail CI.
- **AC4** (eslint fails on raw selectors or `waitForTimeout` in specs): verified by temporarily adding `page.locator('x')` to a spec during development — eslint flagged it with the POP message; removed and eslint passes. Also bans `request.fetch(...)` per RR-3VPYE.
- **AC5** (each test starts its own backend on a unique port):
`workers: 2` in CI, unbounded locally. `findFreePort` + retry loop on startup
failure for port TOCTOU. `waitForExit` after SIGTERM so the next test's port
isn't reused before the old server exits.
- **AC6** (unique coverage ported): dashboard Critical-Issues, relation-cards batch save + unsaved-badge, dark-mode toggle, checkbox rendering, analyze page + API, conflicts page + API, entity-detail sections & relations, forms validation + inline creation + default-picker save (BUG-UNEBR regression), markdown editor, keyboard shortcuts, status-bar, settings API — all have named equivalents and pass.

**Design-review findings addressed:**
- 3 critical (RR-B8GJT, RR-17XTS, RR-K6DJL): fixed in fixtures.
- 6 significant (RR-BZUH5, RR-3VPYE, RR-3DJ2C, RR-F3IA3, RR-LWG6W, RR-J9BIT): fixed.
- 4 minor/nit: addressed (RR-GX4BK, RR-26RE6, RR-2TFDO, RR-SG0LP, RR-MS1FM).
- 4 minor/nit deferred with reason (RR-0RDB4, RR-65GPQ, RR-9WOSL, RR-VKXY2, RR-V63DT).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
