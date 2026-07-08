---
id: REV-JVFCTM
type: review-checklist
title: 'Review: Kanban board silently drops entities beyond page 1 (no pagination in KanbanView)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — Go: `go test ./...` exit 0. Frontend: 77 files / 1217 tests pass.
- [x] Lint clean (`just lint`) — golangci-lint 0 issues; ESLint 0 errors (4 pre-existing warnings on touched files: `max-lines` on KanbanView.vue, type-assertions in the old test file).
- [x] Coverage maintained (`just coverage-check`) — all package floors PASS, total 77.2%. (No Go code changed; frontend has no coverage enforcement.)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — verdict: fix fundamentally sound, contracts honored; 2 significant + 1 minor + 1 nit findings.
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — RR-MZ7NJU (AbortSignal threaded through listEntities/listAllEntities/boardQuery; superseded loops now cancel), RR-G5435I (empty-page guard breaks the loop, has_more preserved). Both pinned by new unit tests (16 tests in entitiesListAll.test.ts + pagination component tests, all green after fixes).
- [x] Self-reviewed the diff for unrelated changes — diff touches only the 5 files of this fix plus tickets/ workflow entities.

**Review Responses:** Code review: RR-MZ7NJU (significant, addressed), RR-G5435I
(significant, addressed), RR-XRCIDE (minor, wont-fix with reason), RR-DB3W6Q
(nit, addressed). RR-1IBKZ0 (design-review deferral) superseded and addressed by
the RR-MZ7NJU fix. Design review responses: RR-QD46GS, RR-0Y3Q6T, RR-5YVXMK,
RR-JUVDUW (addressed), RR-7YDNSN (wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-DNOXI1)

**Acceptance Status:**

- Board renders ALL entities of the type, not just page 1 — **PASS**: live verification with 46 entities (reporter's scenario; API pages at 25, board shows 46 across 4 columns) and 130 entities (2 live page requests, 130 unique cards, TASK-130 present). Component test pins a page-2-only entity landing in its column.
- No duplicates when pages skew — **PASS**: dedupe-by-ID unit test (later page wins); live check `new Set(ids).size === 130`.
- Truncation is user-visible, never silent — **PASS**: cap-hit component test asserts the banner ("Showing 50 of 9999 items — the board is incomplete."); complete fetches show no banner (asserted in both live runs and tests).
- Existing board behavior unchanged (optimistic drag-drop, `_actions` gating, SSE refetch) — **PASS**: same cache key and response shape; full existing KanbanView suite green (with mock repointed to listAllEntities).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing config or API surface changed)
- [x] ~~User-facing documentation updated~~ (N/A: behavior now matches what docs already imply — the board shows all entities)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** pending
