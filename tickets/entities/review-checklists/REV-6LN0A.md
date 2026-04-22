---
id: REV-6LN0A
type: review-checklist
title: 'Review: Consolidate frontend/e2e into /e2e with fixes and CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; `cd e2e && npm test` — 178 pass, 5 intentionally skipped)
- [x] Lint clean (`just lint`; `cd e2e && npm run lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code modified; CI verifies)
- [x] Typecheck clean (`cd e2e && npm run typecheck`) — added as part of this ticket.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Round 1 (design review): RR-B8GJT, RR-17XTS, RR-K6DJL, RR-BZUH5, RR-3VPYE,
RR-3DJ2C, RR-F3IA3, RR-LWG6W, RR-J9BIT, RR-GX4BK, RR-0RDB4, RR-26RE6, RR-SG0LP,
RR-2TFDO, RR-65GPQ, RR-9WOSL, RR-MS1FM, RR-VKXY2, RR-V63DT.

Round 2 (code review): RR-FQ302, RR-ZK8Y7, RR-3Z7EI, RR-3AERY, RR-UKYG7,
RR-UQ225, RR-F5P1L (criticals); RR-M0099, RR-EVS0T, RR-E417A, RR-47YK1,
RR-WB5VS, RR-0O8JS, RR-BJDDA, RR-IIFS0, RR-OK9RH, RR-XZYX7, RR-XZ8D8, RR-Y33PH
(significants); RR-NKBS3, RR-7BLSU, RR-CNV9S, RR-TH4OJ, RR-ZGW3D, RR-M3D1J,
RR-47148, RR-2A7MX, RR-LO36V, RR-YBW8N, RR-849EB (minors/nits).

All criticals and significants are either `addressed` or `deferred` with a
documented reason.

Spawned BUG-9RANL to track the test-harness gap that forced the checkbox-toggle
test to be skipped.

## Acceptance Verification

**AC1** — `/frontend/e2e/` deleted, `frontend/package.json` has no Playwright:
**PASS**.

**AC2** — `npm test` in `/e2e/` passes: **PASS** (178 passed, 5 skipped
intentionally).

**AC3** — CI `e2e` job gates `build`: **PASS**.

**AC4** — eslint fails on raw selectors / waitForTimeout / request.fetch in
specs: **PASS**.

**AC5** — Each test starts its own backend on a unique port, parallelised:
**PASS**.

**AC6** — Unique coverage from `/frontend/e2e/` present: **PASS**.

## Documentation (refactor: no user-facing docs)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal test-infrastructure refactor)
- [x] `frontend/CLAUDE.md` updated (removed the E2E Tests section and architecture diagram)
- [x] `e2e/tests/AGENTS.md` created documenting Page Object contract and fixture usage

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (skipped tests reference the tracking BUG)
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ — PR to be opened by the user when ready to merge
- [x] ~~All CI checks pass~~ — Local equivalents green; CI will run on push
- [x] ~~PR URL documented below~~ — pending user-initiated push

**PR:** not yet created; branch `feat/consolidate-e2e` ready.
