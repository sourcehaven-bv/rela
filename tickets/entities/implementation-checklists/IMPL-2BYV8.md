---
id: IMPL-2BYV8
type: implementation-checklist
title: 'Implementation: Multi-step (wizard) forms with conditional steps'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — Go: `dataentryconfig/validate_test.go` (wizard cases), `conditionlint/conditionlint_test.go`; Frontend: `useFormWizard.test.ts` (12)
- [x] Integration tests written (full flow) — `e2e/tests/wizard.spec.ts` (6 tests: steps render, visible_when reveal, next/back + ?step= sync, refresh deep-link, required_when gating, hidden-branch pruning)
- [x] Happy path implemented — Go `Form.Steps`; frontend `useFormWizard` + `DynamicForm` wizard branch + `FormFieldList`; `rela validate` condition lint
- [x] Edge cases from planning handled — steps-xor-flat, clamp out-of-range/invalid `?step=`, hidden step shrink → clamp, hidden-branch values dropped from payload, malformed condition → warn+false
- [x] Error handling in place — parse errors surface via `rela validate` (loud) + engine console.warn; nothing swallowed silently

## Test Quality

- [x] Using fixture builders/factories — Go table tests with inline configs; e2e uses the shared fixture project (`task_wizard` form)
- [x] No hardcoded values where object is in scope
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object (e2e reads back via `api.getEntity`)

## Manual Verification

- [x] Feature manually tested end-to-end — via the 6 passing e2e tests against the built server (real SPA + API)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

AC coverage (all 5 pass in `e2e/tests/wizard.spec.ts`):
1. Ordered titled steps → "renders steps and starts on the first step".
2. Show/hide + required by earlier field → "visible_when reveals a conditional step" + "required_when blocks Next…".
3. Next/back + per-step validation + full validation on submit → "next/back navigation…" + "required_when blocks Next…" + "submits a wizard…".
4. Step in URL, refresh-safe → "refresh returns to the encoded step" + `?step=` assertions.
5. Single-page unchanged, opt-in → whole existing `forms.spec.ts` + full frontend suite green (1322).

Checks run:
- Go: `go build ./...` OK; `dataentryconfig`, `conditionlint`, `projectsetup`, `dataentry` tests pass; `just arch-lint` clean.
- Frontend: `npm run test:run` → 1322 passed; `npm run typecheck` clean; eslint clean on changed files.
- e2e: `npx playwright test wizard.spec.ts` → 6 passed.
- CLI: manually verified `rela validate` flags a typo'd `form.has_procesors` reference and a `form.x ==` parse error, and passes a corrected config.

**Bug found during implementation:** the condition engine's `resolveRef`
short-circuited on `hasOwnProperty` before reading an absent key, so a condition
referencing a not-yet-set field never registered a Vue dependency and wouldn't
re-evaluate when the field was later set (the `form.done` reveal stayed stale).
Caught by the e2e (unit tests reassigned the whole ref, masking it). Fixed by
reading the property before the own-property check.

## Quality

- [x] Code follows project patterns — composable mirrors `useUrlFilterSync` (seed/replace/echo); condition lint sits above `dataentryconfig` so the config package stays predicate-free (arch-lint enforced); `FormFieldList` extraction dedups the field-render loop
- [x] Checked for DRY opportunities — extracted `FormFieldList` (was duplicated flat+sections blocks), `validateFormField`/`validateFormRelation` (reused by flat + steps), `resolveRelationWidgets` (flat + steps)
- [x] No security issues — client conditions are UX-only (server re-validates every write); condition property lookups stay prototype-pollution-safe (own-property only, preserved through the reactivity fix)
- [x] No silent failures — bad conditions surface at `rela validate` and warn at runtime
- [x] No debug code left behind — throwaway probe specs removed
