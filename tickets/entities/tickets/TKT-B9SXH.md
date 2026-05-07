---
id: TKT-B9SXH
type: ticket
title: RelationCards immediate-persist mode for auto-save forms
kind: enhancement
priority: medium
effort: m
status: ready
---

## Summary

Make `RelationCards` persist row changes immediately (per
add/remove/property-edit) when the parent form is in `auto_save: true` mode,
instead of accumulating a cumulative diff and saving on submit.

## Why split out

Carved off TKT-18JS6 (form-level auto-save). The cranky design reviewer of that
ticket flagged that the proposed approach — calling `savePendingRelationCards`
eagerly after each `cards-changed` event — is broken:

- `savePendingRelationCards` ends with `saveGeneration.value++` (DynamicForm.vue:432). The form template uses `saveGeneration` as the `:key` of `<RelationCards>` (lines 575, 606), forcing a remount. With auto-save firing per change, RelationCards remounts mid-interaction — destroying SlimSelect state, popovers, scroll, focus, and pending row edits.
- `RelationCards.emitUpdate` emits the *cumulative* diff (added/removed/updated as accumulators since last load). Eager save would re-emit the same entries on every change → duplicate `createRelation` / `updateRelationProperties` calls.

This needs a real refactor, not eager-save: either an `autoSave` prop on
RelationCards with a self-resetting per-row diff, or a `flush+reset` method that
bypasses the saveGeneration remount.

## Depends on

TKT-18JS6 (form-level auto-save infrastructure: indicator, dirty registry,
response merge).

## Out of scope

Form-level field auto-save (TKT-18JS6).
