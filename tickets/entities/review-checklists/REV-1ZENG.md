---
id: REV-1ZENG
type: review-checklist
title: 'Review: Lint rule to flag pure-API test patterns in e2e specs'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — N/A locally for this change; the diff touches only `e2e/eslint.config.js` and `e2e/tests/AGENTS.md`. No Go test code changed; e2e Playwright runs unchanged. CI will run the full suite.
- [x] Lint clean — `just lint` (golangci-lint) reports 0 issues; `cd e2e && npm run lint` exits 0 with no errors on the unmodified spec tree.
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: the change is an eslint config + a docs paragraph; no executable code paths added or removed.)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer was invoked manually on this branch)
- [x] All critical review-responses addressed (none were classed critical)
- [x] All significant review-responses addressed (RR-C7AI9)
- [x] Self-reviewed the diff for unrelated changes — diff is exactly two files: `e2e/eslint.config.js` (+25 lines), `e2e/tests/AGENTS.md` (+13 -3 lines, plus the new fifth bullet)

**Review Responses:**

- RR-C7AI9 (significant) — addressed: dropped the `test()`-ancestor restriction; rule now bans `api.rawRequest` anywhere in `tests/**/*.spec.ts`, matching the `request.fetch` precedent.
- RR-UTD3R (minor) — addressed: added a second selector entry for `api['rawRequest'](...)` bracket access.
- RR-1O10G (minor) — addressed: rewrote the eslint message to two sentences explaining what to do instead.
- RR-GYIPB (minor) — addressed: folded `api.rawRequest(...)` into the `Page Object Pattern (enforced)` bullet list as a fifth banned primitive, with a pointer to the dedicated section.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| AC1 | PASS | Canary spec with the user's `rawRequest('GET', '...?direction=incoming')` example fires `no-restricted-syntax` with the new message. |
| AC2 | PASS | `cd e2e && npm run lint` exits 0 on the unmodified spec tree. |
| AC3 | DEPRECATED → re-verified as global ban | Original AC said "hooks exempt." Cranky review surfaced that the test()-ancestor restriction created an inconsistency with the `request.fetch` precedent, so the rule was simplified to ban globally in specs (matching `request.fetch`). Verified empirically: `api.rawRequest` in a `beforeEach` body now correctly fires. No spec uses `rawRequest` today, so this is forward-looking only. |
| AC4 | PASS | `e2e/tests/AGENTS.md` updated: new bullet at line 15, new dedicated section "API-only assertions belong in Go" before "Security canary lives in Go." |
| AC5 | PASS | Same as AC1 — fires on the literal example. |

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: the only docs touched are dev-tooling docs at `e2e/tests/AGENTS.md`, which are part of this PR; a separate docs-checklist would be ceremony for a one-paragraph change.)
- [x] User-facing documentation updated — `e2e/tests/AGENTS.md` only. No user-facing surface (CLI / API / guide / tutorial) is affected.
- [x] ~~Docs-checklist marked as done~~ (N/A — see above)

**Docs Checklist:** N/A — dev-tooling docs are part of this PR.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- to be filled -->
