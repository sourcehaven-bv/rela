---
id: FEAT-0YL031
type: feature
title: Inline entity creation from relation fields
description: 'Let a user creating or editing an entity spawn a new target entity directly from a relation field — in a modal, without navigating away and losing the in-progress draft — and have it linked on return. The affordance is derived, not configured: it appears when the principal may create that entity type AND a non-edit form is registered for it. The nested form is the same DynamicForm machinery as a top-level create form (fields, widgets, templates, validation, wizard steps, affordance gating). Both RelationPicker and RelationCards participate.'
status: proposed
---

Let a user creating or editing an entity spawn a **new** target entity directly
from a relation field — in a modal, without navigating away and losing the
in-progress draft — and have it linked on return.

## Derived, not configured

The affordance appears for a target type when **both** hold:

- the principal may `create` that entity type; and
- a non-edit-mode form is registered for that type.

There is no per-relation config knob. `FormRelation.allow_create` and
`FormRelation.create_form` are removed — an operator controls the offer by
registering (or not registering) a create form for the type, and by ACL. This
matches how side-panel section "Add" buttons already derive their form via
`createFormForType`.

Because a create form is a full form definition, an operator who wants a *small*
inline form simply writes one with fewer fields — the form definition is the
"limited fields" mechanism, so no separate knob is needed.

## Same form machinery

The nested creation form is driven by the same `data-entry.yaml` form definition
as a top-level create form (fields, labels, widgets, templates, validation,
wizard steps, affordance/dry-run gating), not a parallel ad-hoc renderer.

Both relation widgets participate: `RelationPicker` (IDs-only) and
`RelationCards` (edge-meta), so a relation with edge properties can be created
and annotated in one pass.
