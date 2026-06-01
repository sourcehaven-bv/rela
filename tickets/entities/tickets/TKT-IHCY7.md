---
id: TKT-IHCY7
type: ticket
title: Generalize inline-edit plumbing across widgets
kind: enhancement
priority: high
effort: m
status: backlog
---

## Goal\n\nFactor the existing markdown-checkbox PATCH+splice flow (commit ea08ef1) into a generic 'inline-edit a property' handler that any widget can plug into.\n\n## Scope\n\n- Extract `useInlineEdit(entity, property, widget)` composable from `EntityDetail.vue`'s `contentClick` flow.\n- The composable: takes new value → calls `updateEntity(type, id, { [property]: newValue })` → splices response back into reactive state → handles failure (revert + toast).\n- Per-field permission gating uses the existing `_fields` affordance mechanism already used by forms.\n- Widgets opt into inline-edit by accepting an `onChange` prop and calling it.\n- The existing checkbox-in-markdown behaviour continues to work, refactored to use the new composable internally.\n\n## Non-goals\n\n- No config surface yet to flip view sections into inline-edit (that's the next ticket).\n- No new widget types.\n- `content` display mode autosave deferred to the final ticket.\n\n## Why\n\nMakes inline-edit a property of the widget+mode combination, not a special case per widget.
