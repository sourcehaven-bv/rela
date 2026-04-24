---
id: REV-NZMA9
type: review-checklist
title: 'Review: Honor return_to as a back affordance on non-form screens'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` clean; frontend vitest 503 passed; e2e 158 passed + 1 skipped)
- [x] Lint clean (`just lint` — 0 issues; frontend `npm run lint` — 0 errors, 66 pre-existing warnings)
- [x] Coverage maintained (`just coverage-check` — package + total floors PASS)
- [x] Arch-lint clean (`just arch-lint` — added e2e to exclude list)
- [x] Type check clean (`npm run typecheck` — no errors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (0 critical findings)
- [x] All significant review-responses addressed (3 findings, all resolved)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

| RR | Severity | Status | Resolution summary |
|----|----------|--------|---------------------|
| RR-1614T | significant | addressed | Added `isAnyModalOpen()` guard at top of CustomView's handleKeydown |
| RR-5HUNB | significant | addressed | Fixed token typo + added third-pass byte-equality idempotency check |
| RR-D3K32 | significant | addressed | Replaced href-only regex with whole-tag regex + attribute-level parser; 8 new tests cover adversarial attribute orders |
| RR-VLM8A | minor | addressed | Replaced ToLower switch with EqualFold |
| RR-2297B | minor | addressed | Wrapped BackButton + h1 in `.header-left` on KanbanView, AnalyzeView, EntityList |
| RR-BHBWJ | minor | addressed | Added data-testid="back-button" to BackButton.vue; e2e locator keys on testid |
| RR-A26AZ | minor | wont-fix | Pre-existing behavior preserved; '/' is a valid router target with dashboard redirect |
| RR-1HIWF | nit | wont-fix | Audit-trail split is a future telemetry concern, not a rewriter concern |
| RR-ICIU9 | nit | wont-fix | Intentional UX change; documented in "Back navigation" section |

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| AC1 | PASS | `TestRewriteDocumentLinks` — 10+ cases for list/entity/kanban/non-route paths |
| AC2 | PASS | `useBackTarget.test.ts` — 20 cases incl. hostile-input fallthrough |
| AC3 | PASS | `useScopeNavigation.test.ts` — Prev/Next query preservation; manual smoke of EntityView + CustomView |
| AC4 | PASS | e2e round-trip — feature→bug→Back lands on feature with ?doc= preserved |
| AC5 | PASS | `back-button.spec.ts` — ListView renders + navigates via Back |
| AC6 | PASS | no change to DynamicForm; existing `forms.spec.ts` passes unchanged |
| AC7 | PASS | `back-button.spec.ts:22` — behavioural round-trip (click + waitForURL) |
| AC8 | PASS | `TestIsSafeReturnPath` — 4 lowercase cases added |
| AC9 | PASS | `TestHandleV1Documents_CacheInvariance` — dual-render + disk inspection |
| AC10 | PASS | `TestRewriteDocumentLinks_Idempotent` — same-returnPath byte-equal + different-returnPath replace + third-pass equality (RR-5HUNB strengthened) |

## Documentation (enhancements only)

- [x] User-facing documentation updated
- [x] `docs/data-entry.md` + `docs-project/.../GUIDE-data-entry.md` "Links in rendered documents" section revised
- [x] New "Back navigation" subsection documents the precedence rule
- [x] `frontend/CLAUDE.md` package-layout table mentions BackButton / useBackTarget / styles/

Docs-checklist: not created (this is a small enhancement where the existing
data-entry guide's "Links in rendered documents" section is the only doc
affected; per project convention, a separate docs-checklist is overhead for a
single-file doc touch).

## Final Checks

- [x] Commit messages explain the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (RR-A26AZ explicitly deferred with reason)
- [x] Ready for another developer to use — composable + component documented with usage examples in docstrings

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- populated after /pr -->
