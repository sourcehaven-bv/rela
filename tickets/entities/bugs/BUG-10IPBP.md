---
id: BUG-10IPBP
type: bug
title: Selection relation not saved when creating an entity in data-entry (edit works)
description: A relation rendered as a selection widget in the data-entry create form is not persisted when the entity is created; the same field saves correctly on the edit form.
priority: medium
why1: 'On the data-entry create form, an incoming-direction relation picker (widget: multi-select/select with `direction: incoming`) never emits the user''s selection, so it''s absent from the create POST body.'
why2: RelationPicker.emitIncomingDiff() returns early when `incomingLoaded` is false, and on create `incomingLoaded` is always false because loadIncomingValue() short-circuits when there is no entityId.
why3: 'The `direction: incoming` code path was designed around the edit form (diff a loaded snapshot of existing incoming edges); the ''load-failure-cannot-wipe'' guard (TKT-GFQK) treats ''not loaded'' identically to ''load failed'' and suppresses all emits, including pure additions on a brand-new entity that has no snapshot to load.'
why4: 'Create mode for incoming pickers was never exercised end-to-end: incoming pickers were added for the edit flow, and no test (unit or e2e) covers selecting an incoming peer on the create form.'
why5: The two directions use asymmetric emit channels (outgoing emits immediately via update/update:types; incoming emits a loaded-snapshot diff), and create-mode was only validated for the outgoing/default picker path, so the incoming-on-create gap went unnoticed.
prevention: 'When a widget has a mode-dependent code path (create vs edit, keyed on entityId presence), both modes must be exercised end-to-end before merge. The incoming-direction picker was only tested on edit. Systemic fix: e2e/unit coverage now asserts create-mode persistence for incoming pickers (see incoming-picker-create-persist-test). More broadly, the ''load-failure-cannot-wipe'' guard should distinguish ''nothing to load (new entity)'' from ''load failed'' rather than collapsing both to inert — a create-mode empty baseline is a valid loaded state, not a failure.'
status: done
---

In the data-entry create form, a relation rendered as a selection widget is not
persisted when the entity is created. The same relation field saves correctly on
the edit form. So creating an entity and picking a relation value drops that
relation; only after saving and re-editing does setting the relation stick.

**Expected:** a relation selected in the create form is saved as part of entity
creation, matching the edit-form behavior.

**Steps to reproduce:**
1. Open the data-entry create form for an entity type that has a relation field shown as a selection widget.
2. Select a value in that relation field.
3. Fill required fields and create the entity.
4. Observe the created entity has no relation.
5. Edit the same entity, select the same relation value, save — the relation is now persisted.
