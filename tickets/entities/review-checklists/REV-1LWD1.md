---
id: REV-1LWD1
type: review-checklist
title: 'Review: Extract shared widget registry from FieldRenderer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`npm run test:run` — 900 passed, 53 test files)
- [x] Lint clean (`npm run lint` — 0 issues in new/changed files)
- [x] Coverage maintained — widgets dir 95.5% stmts / 100% lines; the lone `coverage:check` violation (`src/stores/schema.ts`) reproduces on a clean develop tree with this branch's changes stashed, so it's a pre-existing flake unrelated to this work.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: zero critical findings)
- [x] All significant review-responses addressed (RR-MIT7R, RR-U98VI, RR-RPC4X — all `addressed`)
- [x] Self-reviewed the diff for unrelated changes (reverted incidental `frontend/vite.config.js` rewrite from `npm run build`; only intentional widget-registry files in the commit)

**Review Responses:**

Significant (3, all addressed):

- RR-MIT7R — transitions-info spacing 14px→8px → fixed by `margin-top: 6px` on `.transitions-info`
- RR-U98VI — `.checkbox-wrapper :deep(input)` too broad → tightened to `:deep(input[type='checkbox'])`
- RR-RPC4X — `.form-field :deep()` reaches future widget descendants → moved input/textarea/select typography into each widget's own scoped style; FieldShell now owns only label/help/error chrome

Minor (5, mix of addressed and deferred):

- RR-17DL8 — dead `isRrule ? help : undefined` conditional → addressed (always pass help)
- RR-A3C4S — inconsistent emit typing → addressed (all widgets emit `[value: unknown]`)
- RR-86ZGD — six copies of `stringValue` computed → addressed (extracted `useStringValue` composable)
- RR-CRG00 — string-equality magic on resolved widget name → deferred (real improvement but expands `WidgetRegistry` contract; revisit when TKT-UD7YR or TKT-HOIX1 need per-widget layout-position)
- RR-15UQS — four small test gaps → addressed (added all four)

Nit (2, both deferred):

- RR-019AK — no frontend widget-name constants → deferred (cosmetic; tests import widget refs by identity)
- RR-DA9PP — `defaultRegistry` constructed at module load → deferred (no real bug today per reviewer)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. ✅ `defaultRegistry` resolves correct widget for every (propertyType, widget, list, values) combo present in repo configs — `registry.test.ts` enumerates the matrix; `defaultRegistry` tests resolve every production widget by name and by type default.
2. ✅ `FieldRenderer.vue` no longer contains per-widget `v-if` — gutted to thin glue that resolves and slots into `FieldShell`. Verified by reading the diff and the file.
3. ✅ `DynamicForm.vue` unmodified — confirmed by `git diff` and by the existing form e2e tests passing without change.
4. ✅ Each of the 8 in-scope widgets renders identically before/after — verified by (a) the existing `FieldRenderer.test.ts` (5 affordance/transition tests pass unchanged); (b) 49 new widget-level tests; (c) two browser smoke runs against the tickets project — the post-fix screenshot is visually indistinguishable from the pre-fix run.
5. ✅ Tests construct isolated `defineWidgetRegistry()` — `registry.test.ts` builds isolated registries and registers stubs without module mocking.
6. ✅ `cards` continues to work via its existing path — explicitly excluded from the registry; `RelationCards.vue` untouched.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created~~ (N/A: internal refactor, no user-facing API change)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (17/17 non-self-referential checks green; the `Rela Tickets` validation gate fires on review+in-progress workflow states, resolved by transitioning ticket and checklist to `done` once the PR is up and reviewed)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/848
