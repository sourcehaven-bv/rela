---
id: RR-6OIIOE
type: review-response
title: guardWriteBack was never called in production — the save path used the raw emit
finding: MilkdownEditor exposed guardWriteBack as an optional `guardedValue()` accessor, but DynamicForm saved through `@update:model-value="updateContent"`, which carried the raw serialization from Milkdown's markdownUpdated listener. Every caller of guardedValue() in the repo was a test. The ticket's headline invariant — opening an entity and saving it emits nothing — was enforced by a function with no production caller.
severity: critical
resolution: 'Moved the guard ONTO the emit rather than beside it. The listener now routes through decideEmit(), a pure function extracted to writeBackGuard.ts, and there is no unguarded path out of the component. Verified by reintroducing a raw emit alongside the guarded one: the new test ''emits a genuine edit through the guard'' fails, then passes on restore.'
status: addressed
---

## Finding

`MilkdownEditor.vue:628` exposed `guardedValue`, and every caller in the repo
was `MilkdownEditor.test.ts`. `DynamicForm.vue:2037` binds
`@update:model-value="updateContent"`, so the save path took the raw markdown
straight from the `markdownUpdated` listener.

The design named round-trip suppression as its first constraint and shipped it
as an opt-in side door.

## Why the tests missed it

Every emit assertion was negative (`expect(emitted(...)).toBeUndefined()`), and
Milkdown's listener is debounced and does not fire for a programmatic dispatch
under happy-dom. A negative assertion against a channel that never fires cannot
distinguish "correctly suppressed" from "never ran". The guard's own tests
reached past the component's wiring and called the exposed method directly,
which proves the function works, not that the feature does.

## Resolution

The guard now sits on the emit. `decideEmit()` in `writeBackGuard.ts` is a pure
function taking `(markdown, original, settled, dirty)` and returning `ignore` /
`emit` / `report-drift`, so the decision is testable without mounting an editor.

New tests drive the registered `markdownUpdated` callback directly, which
exercises the real path. Confirmed non-vacuous: adding a raw
`emit('update:modelValue', markdown)` beside the guarded one fails `emits a
genuine edit through the guard`.
