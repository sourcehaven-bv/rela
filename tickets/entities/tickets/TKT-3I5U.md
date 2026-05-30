---
id: TKT-3I5U
type: ticket
title: 'Create-form field affordances: default _fields verdicts for an unsaved entity'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

After BUG-Q60V, the v1 create path 403s a write to a hidden / read-only /
option-filtered field, matching the PATCH gate. But the SPA create form has no
affordance source: `DynamicForm` renders all fields and options in create mode
(`DynamicForm.vue`: *"In create mode, no entity is loaded — render
everything"*); `_fields` only rides on a fetched entity GET. So a user can fill
a field the server then rejects, surfacing as a save error rather than a
disabled input.

Close the UX gap:

1. Expose default field / option / relation verdicts for an entity **type** before any instance exists (e.g. `_fields` on the collection / new-entity response, or a dedicated endpoint), evaluated against the principal's global roles + a candidate entity with no ID. Relation- and entity-scoped predicates fail closed (no instance), consistent with the server's create-time evaluation in `handleV1CreateEntity`.
2. Wire `DynamicForm` create mode to disable read-only fields, hide hidden fields, and filter enum options the same way edit mode already does (`isFieldReadonly` / `optionVerdictsFor`).

Note: this changes the wire contract (new affordance surface) — coordinate per
the api-reference verb rules.

Follow-up to BUG-Q60V (server-side bypass fix shipped without the create-form
gating).
