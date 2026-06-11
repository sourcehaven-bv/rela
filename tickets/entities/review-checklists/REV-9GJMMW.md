---
id: REV-9GJMMW
type: review-checklist
title: 'Review: Pinia Colada foundation: targeted SSE invalidation + KanbanView migration'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (965 unit tests at PR time, 987 after the stacked review fixes; 15 kanban E2E specs against the built rela-server)
- [x] Lint clean (0 errors, 77-warning baseline)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: frontend coverage ratchet removed in PR #944)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer ran over the full stack diff: 0 critical, 0 significant, 5 minor)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes (vite.config.js regeneration noted in PR body; removal tracked separately)

**Review Responses:** RR-IVBO9K (minor, deferred — optimistic-mutation unit
coverage moves to the shared helper extracted in the EntityList migration slice)

## Acceptance Verification

- [x] Each acceptance criterion tested (targeted invalidation: useEvents unit tests assert per-type keys; background refetch without spinner: isPending-gated template + kanban E2E; optimistic drag-drop with rollback+toast: kanban drag E2E + code review)
- [x] Test evidence documented in implementation checklist (no implementation checklist was auto-created for this ticket — created directly in `in-progress`; evidence lives in PR #953's description and this checklist)

**Acceptance Status:** All four ticket scope bullets PASS — plugin wired
(main.ts), targeted SSE invalidation (useEvents + 3 new unit tests), KanbanView
on useQuery (15/15 kanban E2E), drag-drop useMutation with copy-on-write +
rollback + toast (E2E drag spec + cranky review confirmed rollback identity
check correct).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal architecture change, no user-facing behavior beyond removed spinner flicker; the migration arc is documented on FEAT-XY2D1L)
- [x] ~~User-facing documentation updated~~ (N/A: same)
- [x] ~~Docs-checklist marked as done~~ (N/A: same)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use (queries/entities.ts documents the key hierarchy and migration pattern for the next view)

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (green after this ticket transitions to done; the Rela Tickets gate requires non-in-progress/review statuses)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/953
