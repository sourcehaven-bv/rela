---
id: REV-HYACI21
type: review-checklist
title: 'Review: E2E flakes: contract test SSE windows and form page helpers'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no Go or SPA source change; the changed suites pass: contract loop 34/34, forms and wizard e2e 185/185)
- [x] ~~Lint clean (`just lint`)~~ (N/A: no Go change; eslint and tsc are clean for e2e and frontend/contract)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: commentlint checks Go comments; no Go change)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go change)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-33TQ9GX, RR-3DFHGP2, RR-49YJYBO, RR-CZ0O1QA,
RR-DQWK7Q2, RR-VWD3U7R, RR-WSV1YH0, RR-XNOW58C. One significant, addressed. One
nit is wont-fix. The security review found nothing.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Contract flake reproduced with seed 516079759 and fixed: PASS.
- Contract loop on the final code, 2 recorded and 32 random seeds: 34/34 PASS.
- forms.spec.ts:45, forms.spec.ts:119 and wizard.spec.ts:80 under load:
185/185 PASS.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix in tests)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix in tests)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix in tests)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
