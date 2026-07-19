---
id: IMPL-ZPQHEW
type: implementation-checklist
title: 'Implementation: Machine-aware status control: surface _transitions on the wire + SPA performable-transition UI + entry-locked create field'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (dry-run create + GET through the serializer)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using focused fakes (fakeTransitionResolver, labelMeta) for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: fixed enum values)
- [x] Property comparisons use the verdict/wire object

## Manual Verification

- [x] Each acceptance criterion verified via automated test scenario

**Verification Evidence:**

Backend (`go test ./internal/dataentry ./internal/statemachine ...` — all pass):
- AC1: `TestTransitionsWire_GETCarriesTransitions` — GET carries `_transitions`
with full wire mapping (to/label/guard/allowed/reason);
`TestTransitionsWire_AbsentWithoutTransitionResolver` — absent under Nop.
- AC4: `TestTransitionsWire_CreateLocksMachineField` — dry-run pins the field to
entry, marks writable=false, strips `_transitions`.
- AC5 label fallback: `TestPerformable_SurfacesLabel`; `TestEntryValue`.
- Drift guard untouched: existing `TestPerformable_MatchesEnforceUpdate` still passes.

Frontend (`npm run test:run` — 1338 pass incl. new):
- AC2: `FieldRenderer.test.ts` routes machine field → StatusControl, non-machine
→ SelectWidget, terminal (empty) → StatusControl.
- AC2/AC3: `StatusControl.test.ts` — only allowed moves shown, action-labeled
(+ raw-value fallback), commit emits target, self-loop excluded, terminal =
inert, disabled = no menu.

Coverage: `just coverage-check` PASS (statemachine 86.5%, all floors met). Lint:
`just arch-lint` OK; golangci-lint 0 issues; eslint 0 errors on new files.

Note on create-lock SPA coverage: the `formData` value-adoption in
`refreshStagedAffordances` is verified at its boundaries (backend
`TestTransitionsWire_CreateLocksMachineField` pins the wire; existing
`FieldRenderer` readonly tests cover the disabled render). A full DynamicForm
mount test was not added — the repo's own convention (FieldRenderer.test.ts
header) avoids the full-mount cost, and the behavior is contract-covered.

## Quality

- [x] Code follows project patterns (optional-capability type-assert like store
HistoryReader; consumer-side TransitionResolver interface; affordance-map wire
idiom)
- [x] Checked for DRY — reused evalEdge drift guard, existing useAutoSave commit
path, existing isFieldReadonly for create-lock; no premature abstraction
- [x] No security issues — `_transitions` is server-computed, read-only hint;
write path re-enforces (attempt-and-recover); no client-side ACL
- [x] No silent failures
- [x] No debug code left behind

**Scope note:** included a small additive `metamodel.TransitionDef.Label`
(user-approved) to satisfy "name options as their transition"; display-only, no
enforcement change, fully backward compatible.
