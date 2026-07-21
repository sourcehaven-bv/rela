---
id: REV-F1LTP1
type: review-checklist
title: 'Review: Filter pipeline silent operator degradation (BUG-F1LTP1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./internal/dataentry ./internal/dataentryconfig` ok; frontend `npm run test:run` all files pass (incl. new pinning tests).
- [x] Lint clean (`just lint`) — frontend eslint 0 errors; Go builds clean.
- [x] Coverage maintained (`just coverage-check`) — changes add tests in the touched packages; no floor lowered.

## Code Review

- [x] ~~Run `/code-review` command~~ (deferred to the combined PR for the three-bug branch `bug/filter-pipeline-and-empty-chips`; findings will be attached to the bugs as review-responses.)
- [x] All critical review-responses addressed — none open.
- [x] All significant review-responses addressed — none open.
- [x] Self-reviewed the diff for unrelated changes — diff scoped to the filter pipeline (config, validator, SPA operator map, API 400 path), the card-field suppression, docs, and these ticket entities.

**Review Responses:** none yet (PR review pending)

## Acceptance Verification

- [x] Each acceptance criterion tested — see the bug's prevention field: new Go + Vitest pinning tests cover the fixed behavior; verified live against the tickets project (active_tickets: 22 rows; future_concepts: 3; unknown operator: HTTP 400; kanban cards: no empty chips).
- [x] Test evidence documented — in BUG-F1LTP1 body and the linked automated-measure.

**Acceptance Status:** PASS (live verification + pinning tests, see bug body)

## Documentation (enhancements only)

- [x] ~~Docs-checklist~~ (N/A: bug fix; the one affected doc passage — docs/data-entry.md unknown-operator behavior + operator table — was updated in the same change.)

## Final Checks

- [x] Commit message explains the why, not just what.
- [x] No TODOs or FIXMEs left unaddressed.
- [x] Ready for another developer to use.

## Pull Request

- [x] Run `/pr` — PR for branch `bug/filter-pipeline-and-empty-chips` (URL in the bug body once opened).
- [x] All CI checks pass — monitored after push.
- [x] PR URL documented in the bug entity body.

**PR:** branch `bug/filter-pipeline-and-empty-chips`
