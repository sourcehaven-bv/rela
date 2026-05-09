---
id: FEAT-XN6JX
type: feature
title: Form auto-save with optimistic UI
summary: 'Per-field auto-save in data-entry forms: debounced PATCH per property + content with optimistic UI, dirty-field SSE protection, FIFO queue, inline error + revert.'
description: Per-field auto-save in data-entry forms with optimistic UI, dirty-field SSE protection, FIFO save queue, inline error + revert affordance. Replaces explicit Save click for forms whose widgets are auto-save-compatible.
priority: medium
status: proposed
---

## Problem

Today the data-entry forms require an explicit Save click. Users routinely lose
work when they navigate away, when they hit a validation error and back-button
out, or when a flaky save round-trip is misinterpreted as success. A keystroke
in the title field shouldn't be at risk of vanishing.

## What this feature delivers

- Per-property save: each field debounces and PATCHes its own slice of the
entity, so a save error on one field doesn't block the others
- Per-edge relation save: the same per-thing-saves-itself approach extended
to relations, using the wire format from TKT-K2VAA / TKT-6WLSW
- Optimistic UI: typed values appear instantly; the indicator says "saving",
"saved", or "error" with a revert affordance
- Dirty-field protection: while a field is dirty, an incoming SSE
`entity:updated` event does NOT clobber the user's in-progress edit
- FIFO save queue: in-flight saves are serialized; later edits to the same
field collapse into one outgoing PATCH

## Open questions

- How does autosave interact with form-level validation errors that need
multiple fields together? (E.g., "either A or B is required.")
- Conflict resolution UX: what happens when a save returns 422 because the
field was edited externally between the optimistic update and the PATCH?
- Should an offline mode queue saves locally? (Probably no — out of scope
for v1.)

## Tickets that implement this

- TKT-18JS6 (planned): per-property + per-content auto-save in DynamicForm
- TKT-B9SXH (planned): RelationCards / RelationPicker integration so widget
edits flow through the same save queue

## Out of scope

- Server-side optimistic-concurrency tokens beyond the existing ETag
- Multi-tab live collaboration (separate concept, far future)
- Offline draft storage
