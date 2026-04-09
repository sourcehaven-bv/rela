---
id: REV-MB3YV
type: review-checklist
title: 'Review: Unify delete confirmation on custom modal and wire Delete shortcut in list & detail views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`npm run test:run` → 361 tests passing)
- [x] Lint clean (`npm run lint` → 0 errors, 20 pre-existing warnings)
- [x] Coverage maintained (`npm run coverage:check` → baseline passes)
- [x] Type check clean (`npm run typecheck`)
- [x] Backend build clean (`go build ./...`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

| ID | Severity | Status | Summary |
|----|----------|--------|---------|
| RR-RLYNL | critical | addressed | `.modal-overlay` class collision → introduced `modalStack.ts` registry |
| RR-PKJ0S | critical | addressed | AC9 rescoped to delete-flow; follow-up TKT-60E9G filed |
| RR-YCR2G | significant | addressed | ConfirmModal now restores focus on close |
| RR-417OY | significant | addressed | EntityDetail keeps modal open on error |
| RR-7W6ZF | significant | addressed | Backspace guarded on entity.value during load |
| RR-6ZJ53 | significant | addressed | Added EntityList.test.ts with 7 integration tests |
| RR-CB08Y | minor | deferred | Focus trap deferred to TKT-X4P99 (class-of-problem fix) |
| RR-792ON | minor | addressed | ConfirmModal generates unique title ID per instance |
| RR-JTOKA | nit | addressed | Unicode ellipsis + computed busyConfirmLabel |
| RR-R7JKO | nit | addressed | Removed empty `<style scoped>` block |

All critical and significant findings are addressed. Two minor findings
deferred with explicit follow-up tickets: TKT-60E9G (remaining
`window.confirm` calls in CommandModal + DynamicForm) and TKT-X4P99 (shared
`useFocusTrap` for all data-entry modals).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| AC1 List uses custom modal | PASS | `EntityList.test.ts` opens modal on delete-button click; no `window.confirm` in delete path |
| AC2 List Delete+Backspace shortcut | PASS | `useListKeyboard.test.ts` (4 tests) + `EntityList.test.ts` (keydown opens modal for selected row) |
| AC3 Detail Delete+Backspace | PASS | Code inspection: `EntityDetail.vue:47-50` guards on entity loaded and sets `showDeleteConfirm` |
| AC4 Detail `e` shortcut | PASS | Pre-existing; `EntityDetail.vue:41-44` unchanged |
| AC5 Busy state | PASS | `ConfirmModal.test.ts > busy state` (4 tests); `EntityList.test.ts > error test` asserts re-enable |
| AC6 Escape/overlay close | PASS | `ConfirmModal.test.ts > emits` (6 tests) |
| AC7 Shortcuts modal lists Delete | PASS | `KeyboardShortcutsModal.vue` updated |
| AC8 Guards when input focused / modal open | PASS | `useListKeyboard.test.ts > input focus / modal handling`; modalStack coordination |
| AC9 No `window.confirm` in delete flows | PASS (rescoped) | Grep confirms no `window.confirm` in delete code; CommandModal + DynamicForm tracked in TKT-60E9G |

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: in-app docs only)
- [x] ~~User-facing documentation updated~~ (N/A: `KeyboardShortcutsModal.vue` is the in-app doc and was updated in scope)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

**Docs Checklist:** N/A — in-app shortcuts modal is the only user-facing
documentation for keyboard shortcuts, and it was updated as part of this
ticket.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (see PR status)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/361
