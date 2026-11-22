---
id: REV-ZS6QU9J
type: review-checklist
title: 'Review: Entity page refresh after actions, step scroll, sidebar flash, run actor, unknown routes and 422 log'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (full vitest, vue-tsc; e2e runs in CI)
- [x] Lint clean (eslint and prettier on the changed files)
- [x] ~~Comment lint gate clean~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained~~ (N/A locally: new code has unit tests; CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-5IHNZKK, RR-Z68ZDB9)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5IHNZKK, RR-Z68ZDB9, RR-8R3MPTI, RR-XIOROEB, RR-Y8KHL4V, RR-43AR8TS, RR-Y6FRBHV, RR-OOIAHIZ, RR-HAF23BB, RR-MKJV56X, RR-E4AOSM3, RR-QP44V19, RR-0ZQ9V55, RR-ISFGP83, RR-IPAZDMJ

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Panels read again after an action or applied proposal: PASS (EntityDetail.actions.test.ts, ChangesPanel, MentionedByPanel, FlowRunsPanel tests)
- Step form scrolled into view and kept there: PASS (keepInView.test.ts, focusRun.test.ts, flow-task-hint.spec.ts)
- No sidebar flash: PASS (Sidebar.builtins.test.ts)
- Readable actor in the run timeline: PASS (flowView.test.ts)
- Not-found page and /login redirect: PASS (notFound.test.ts)
- No console error for a 422: PASS (DynamicForm.refusal.test.ts, FlowRunCard.test.ts)
- Documented: PASS (docs/data-entry.md and GUIDE-data-entry)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/data-entry.md and GUIDE-data-entry)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there
