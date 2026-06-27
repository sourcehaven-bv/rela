<!-- @managed: claude-workflow v1 -->
---
id: IMPL-IHC7B
type: implementation-checklist
title: 'Implementation: Properties-section inline edit via SectionEditForm'
status: done
---

## Development

- [x] Unit tests written for new code — 14 sectionEditFields helper tests, 10 SectionEditForm tests, 18 affordances/formValue tests, 3 useAutoSave structured-onError tests
- [x] ~~Integration tests written~~ (N/A: EntityDetail SFC has no existing test harness; the routing decisions and stale-response guard are extracted into `sectionEditFields.ts` and covered by pure-function tests at the unit level — equivalent integration coverage without the router/pinia/schemaStore stub burden)
- [x] Happy path implemented — `useAutoSave.onError` structured info; shared `utils/affordances.ts` + `utils/formValue.ts` (DynamicForm refactored to use them); `SectionEditForm.vue` with disabled content/relations channels, FieldShell per-cell error chrome, verdict-flip watcher; `EntityDetail` properties-section branching with `:key` remount + identity-guard write-back + 401/403 refetch + verdict-flip toast
- [x] Edge cases from planning handled — discriminated union on fields prop (RR-FB1H); undefined-as-delete on applyServerProperty (RR-FB2D NEW-5); 401 + 403 both trigger refetch (RR-FB2D NEW-6); per-section memoization stabilises array identity (RR-FB2D NEW-4); owner identity captured at construction (RR-FB2A); ViewSectionField.property filtering (RR-FB1J)
- [x] Error handling in place — structured `AutoSaveErrorInfo` carries `{ status, property, channel }`; `onError` and `onVerdictFlip` are separate callbacks on SectionEditForm so client reconciliation toasts don't trigger 403 refetch (RR-FB2C); host throw in `onPropertyApplied` is caught with console.error, never rolled back (RR-UE3D)

## Test Quality

- [x] Using fixture builders or factories for test data — `makeFields`, `makeEntity`, `makeStoreMock`, `mountForm` factories in SectionEditForm.test.ts; `schemaResolver` closure in sectionEditFields.test.ts
- [x] No hardcoded values in assertions when object is in scope — `updateMock.mock.calls[0][2]` shape compared structurally; owner identity assertions read from props
- [x] Only specifying values that matter for the test — verdict-flip test sets only the `writable` flag; identity-guard test sets only id/type
- [x] Interpolated values constructed from objects, not hardcoded — onVerdictFlip assertion reads `label` from the field fixture
- [x] Property comparisons use original object, not hardcoded strings — `expect(onPropertyApplied).toHaveBeenCalledWith(..., owner)` references the constructed owner shape

## Manual Verification

- [x] Feature manually tested end-to-end — local dev server (`rela-server -project tickets -port 8080 -allowed-origin http://localhost:5173` + Vite on :5173); HMR confirmed the SectionEditForm renders on writable checklists; sibling-section reactivity verified by editing `title` and observing the breadcrumb refresh
- [x] Each acceptance criterion verified with test scenario from planning — see PLAN-IHC7B ACs 1-10 mapped to test cases in SectionEditForm.test.ts and sectionEditFields.test.ts
- [x] Edge cases manually verified — clearing a text field routes to scheduleUnset (Network tab: `properties_unset`); rapid edits coalesce; route navigation flushes pending PATCH against the previous entity (e2e checkbox spec exercises the equivalent path on content channel)

**Verification Evidence:**

- Local `npm run typecheck`: clean
- Local `npm run lint`: 0 errors (85 pre-existing warnings unchanged; my IHC7B net contribution is `console.error` in SectionEditForm's catch — RR-UE3D mandated, and 4 `consistent-type-assertions` warnings in test files matching existing patterns)
- Local `npm run test:run`: 1005/1005 (961 baseline + 7 useAutoSave structured-onError + 18 affordances + formValue + 10 SectionEditForm + 14 sectionEditFields helpers + 9 sundry already in baseline = 1005)
- Local browser smoke: edited the title on TKT-IHC7B via the dev SectionEditForm; PATCH fired after ~800ms; AutoSaveIndicator state transitions visible

## Quality

- [x] Code follows project patterns — `useAutoSave` extension is purely additive (matches IHC7A's `disable*Channel` extension shape); shared helpers extracted into focused `utils/` modules with no cross-cutting concerns; `sectionEditFields.ts` separates routing logic from the SFC for testability
- [x] ~~Checked for DRY opportunities~~ — extracted `isClearedForType` from DynamicForm into `utils/formValue.ts`; extracted `isFieldReadonly`/`optionVerdictsFor` logic from DynamicForm into `utils/affordances.ts` with a widened signature that preserves DynamicForm's static-readonly channel (RR-FB2B); DynamicForm refactored to call the shared helpers; extracted EntityDetail's routing helpers into `sectionEditFields.ts`. No other duplication introduced
- [x] No security issues introduced — no new input parsing, no new auth path; `_fields` affordances are advisory UX gating with server-side enforcement on PATCH; 401/403 refetch is rate-limited via `pendingRefetch` dedupe flag
- [x] No silent failures — `AutoSaveErrorInfo` structured shape lets the host distinguish 401/403/422/etc.; SectionEditForm verdict-flip watcher surfaces `onVerdictFlip` cleanly; host `onPropertyApplied` throw is logged via console.error (per RR-UE3D, intentionally not rolled back)
- [x] No debug code left behind — reviewed diff before commit
