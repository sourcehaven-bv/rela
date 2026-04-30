---
id: REV-4VX2W
type: review-checklist
title: 'Review: Replace remaining window.confirm() calls in data-entry UI with ConfirmModal'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `npm run test:run` — 601 tests pass (35 files)
- [x] `npm run typecheck` — clean
- [x] `npm run lint` — 0 errors (74 pre-existing warnings)
- [x] `npm run build` — succeeds
- [x] `just test` — Go-side tests still pass (no backend changes; sanity check)

## Code Review

- [x] cranky-code-reviewer agent invoked
- [x] All findings logged as review-response entities and linked
- [x] All critical and significant findings addressed

**Findings (10 total):**

| ID | Severity | Title | Status |
|----|----------|-------|--------|
| RR-23MD7 | critical | Bulk action path silently dropped errors (executeAction never throws) | addressed |
| RR-8633Z | critical | ConfirmModal host inside v-else can be unmounted during loading/error | addressed |
| RR-AVRIZ | significant | useConfirmHost cannot detect/refuse a duplicate host mount | addressed |
| RR-UWFSY | significant | onConfirm error-handling boilerplate duplicated across call sites | addressed |
| RR-EBA0L | significant | Delete prompt lost `<strong>` emphasis on entity id | addressed |
| RR-4PMY1 | minor | Stale comment in EntityList.test.ts | addressed |
| RR-ZCUZC | minor | `as never` cast in CommandModal.test.ts | addressed |
| RR-0FPXW | minor | useConfirmHost returns mutable reactive state | addressed |
| RR-A6LEV | minor | Dirty-flag clearing order needs comment | addressed |
| RR-QXC3A | nit | `expect(p1).toBe(p2)` tests implementation | addressed |

## Acceptance Verification

- **AC1 (no `window.confirm` left):** PASS — `grep -rn 'window\.confirm\|globalThis\.confirm\|[^a-zA-Z]confirm(' src/` returns only the new `confirm` composable callers, the test file, and the doc comment in `ConfirmModal.vue`.
- **AC2 (CommandModal):** PASS — covered by `CommandModal.test.ts` (4 tests). Title=`"<cmd.label>?"`, message=`cmd.confirm`, confirmLabel=`cmd.label` matching the existing bulk-action UX pattern.
- **AC3 (DynamicForm guard):** PASS — covered by `DynamicForm.guard.test.ts` (6 tests) including the popstate edge case and `router.replace` history-entry preservation.
- **AC4 (`beforeunload` unchanged):** PASS — code unchanged, comment added explaining browsers require native dialog.
- **AC5 (EntityList/EntityDetail migrated):** PASS — both files now use `useConfirm`; their local `<ConfirmModal>` blocks are removed. Existing EntityList integration tests (10) updated to mount via the singleton harness; all pass.

## Quality Improvements From Review

Beyond AC compliance, the review-driven changes hardened the composable:

1. **Singleton invariants:** `useConfirmHost` throws on dupe mount, `confirm()` warns + resolves false when no host is mounted, `state` returned as `readonly`.
2. **Boilerplate extraction:** `withConfirmError(action, msg, uiStore)` lifts the toast-and-rethrow pattern into the composable; call sites are now single lines.
3. **Bulk-action correctness:** `executeAction` now runs outside `onConfirm` so its built-in `Promise.allSettled` error-toasting flow works correctly.
4. **Host robustness:** `<ConfirmModal>` mounted unconditionally at the App root.
5. **UX preservation:** entity-id wrapped in single quotes in delete prompts to compensate for lost `<strong>` styling.

## PR

- [x] ~~PR opened~~ (deferred: user will run `/pr` separately)
- [x] ~~CI green~~ (deferred: tracked on the PR)
