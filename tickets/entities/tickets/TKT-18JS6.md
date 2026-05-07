---
id: TKT-18JS6
type: ticket
title: 'Auto-save for data-entry forms (auto_save: true)'
kind: enhancement
priority: medium
effort: l
status: in-progress
---

## Summary

Add an opt-in `auto_save: true` flag on form configurations. When enabled, field
changes are persisted via per-field debounced PUTs to the existing entity-update
endpoint. The Save button is replaced by a subtle Saving/Saved indicator.
Existing forms keep their explicit-save behavior (no behavior change unless the
flag is set).

## Why this ticket

This is the first of six tickets that together enable the daily-notes UX (see
FEAT-KTP7S and `.ignored/daily-notes-plan.md`). Auto-save is sliced first
because it is **pure infrastructure**: it has independent value for any form,
and it forces us to design the per-field PUT pattern, optimistic-UI conventions,
and dirty-field reconciliation with SSE before anything else depends on them.

## Key design points

- **Per-field PUT** (not whole-form) so partial validation failures don't block other fields.
- **Optimistic UI**: render the change immediately, reconcile on response. Failure = revert + toast.
- **Dirty-field tracking**: SSE invalidations during the dirty window for that field are ignored, so the user's typing isn't clobbered by their own save round-trip.
- **Activates after entity creation only** — the create flow is unchanged (server still needs ID + required fields up-front).
- **Validation timing**: required-field checks fire on blur or navigate-away, not on every keystroke. Auto-save tolerates transient cardinality / required-field violations.
- **RelationCards reconciliation**: existing `RelationCards` immediate-save path needs to be reconciled with the new field auto-save under one mental model (both operate optimistically, both surface errors the same way).

## Out of scope

- Reorderable relations / `order_by` (separate ticket, depends on `type: order`).
- The `relation-list` widget itself.
- Auto-save during entity creation.
