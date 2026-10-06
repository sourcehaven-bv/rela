---
id: BUG-80LVS8
type: bug
title: Create form cannot save a pre-linked peer whose type uses a dashed id_prefix
description: 'A section create button pre-links the new entity to the page entity. When the create form has no field for that relation, DynamicForm derives the peer''s type from its id. getTypeFromId takes the text before the first dash and compares it to id_prefix, so a type whose id_prefix includes the dash (MOD-) or that only sets id_prefixes never matches. The save aborts with ''could not resolve the entity type of every related item''. Once typed, the edge was also written backwards (new --relation--> peer) for link_as=to. Expected: the entity is created and linked as peer --relation--> new.'
priority: high
effort: s
why1: DynamicForm.getTypeFromId returned no type for the peer id, so the pre-linked relation had no type entry and reshapeLegacyToModern aborted the save.
why2: 'getTypeFromId takes the text before the first dash (MOD) and compares it to id_prefix as an exact string. A type configured as id_prefix: MOD- never matches, and id_prefixes is not read at all.'
why3: The helper assumed the dashless spelling. The schema accepts both spellings and the docs use the dashed one. The backend (MatchesID, analyze) and other frontend code (useEntityIDControls, the mention menu) already handle both and read id_prefixes.
why4: The pre-link unit tests use a fixture with a dashless prefix (FEAT), and no e2e test covered a section create whose form lacks a field for the relation.
why5: The frontend has no shared rule for mapping an id to its type, so each caller writes its own prefix match and they drift from the backend's rule.
prevention: One frontend helper (entityTypeForId) maps an id to its type over id_prefix and id_prefixes in either spelling. A link_as=to pre-link rides the payload under the inverse key. Unit tests cover both prefix spellings and the edge direction; an e2e test drives a section create end to end and checks the edge direction on the server.
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Reproduction

Environment: data-entry SPA, any backend.

```yaml
# schema
entities:
  module: { id_type: manual, id_prefix: MOD-, properties: { name: { type: string } } }
  task:   { id_type: sequential, id_prefix: TASK, properties: { title: { type: string } } }
relations:
  contains: { from: [module], to: [task], inverse: contained_in }
# data-entry: the task form has no field for `contains`
views:
  module:
    entry: { type: module }
    traverse: [{ from: entry, follow: contains, collect_as: tasks }]
    sections: [{ heading: Tasks, source: tasks, display: list, create: {} }]
```

1. Open a module's detail page.
2. In the Tasks section press "Task", enter a title, press Create.
3. The dialog stays open. Toast: "Save aborted: could not resolve the entity
type of every related item for "contains". ..."

## Second defect found while fixing

With the type resolved, the edge was written backwards: `task --contains-->
module`, saved with `source_type_not_allowed` / `target_type_not_allowed`
warnings, and the section stayed empty. For `link_as=to` the new entity is the
relation's target, but the prefill put the peer under the canonical relation
key, which the server writes as `new --relation--> peer`.

## Fix

- `frontend/src/utils/entityIdType.ts`: `entityTypeForId` maps an id to its
type over `id_prefixes` or `id_prefix`, in either spelling, longest match first.
`DynamicForm.getTypeFromId` uses it.
- `DynamicForm.linkBodyKey`: a `link_as=to` pre-link rides the create payload
under the relation's inverse key (its own name when symmetric). A relation
without an inverse is linked after create with `direction: incoming`.

## Tests

- `frontend/src/utils/entityIdType.test.ts`
- `DynamicForm.embedded.test.ts`: dashed prefix, inverse key, no-inverse path.
- `e2e/tests/create-prelink.spec.ts`: section create on a `MOD-` entity
creates the task and links `module --contains--> task`.
