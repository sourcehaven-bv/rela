---
id: BUG-KQSOJ2
type: bug
title: Form save drops all relation edits when the entity has a relation the form does not render
description: 'Editing a relation on an edit form aborts with "Some related entities have unknown types" and sends no PATCH whenever the entity carries any relation the form does not render as an outgoing field. Unlike BUG-HOB9BR the failure is permanent rather than transient: the entity GET returns the same untyped key every time, so the advised reload cannot help and the field is simply unsavable.'
priority: high
effort: s
why1: reshapeLegacyToModern returned null because pickerTypes had no entry for a relation the form does not render, so both save paths aborted the entire relations payload and sent no PATCH.
why2: pickerTypes is filled only by a RENDERED RelationPicker via updateRelationTypes, while relations.value is seeded from the entity GET, which returns every relation the entity has.
why3: The only filter between relations.value and the payload excluded cards-managed relations. Nothing asked whether the form renders the relation at all, so unrendered keys rode along untyped.
why4: The form treats the entity full relation map as its own form state. Ownership was never modelled, so a key the form cannot type and must not write was indistinguishable from one it owns.
why5: An all-or-nothing reshape makes one unownable key fail the whole payload, and the error text blamed the data and advised a reload rather than naming the relation, so a permanent config-shaped cause read as a transient loading one.
prevention: 'Model ownership explicitly: a form may only write relation keys it renders as an outgoing field (ownedRelations.ts), so an unrendered relation cannot reach the payload by default. The error now names the offending relation instead of advising a reload that cannot help. Longer term, take the id-to-type map from the entity read (which already carries each linked entity type) rather than reconstructing it from a candidate list, as BUG-HOB9BR prevention already recommended.'
status: done
---

## Description

Editing a relation on an edit form fails with "Some related entities have
unknown types; relation changes were not saved. Reload the form and try again."
and sends no PATCH, whenever the entity has ANY relation the form does not
render as an outgoing field.

Reported on atlas `/form/edit_taak/TASK-7F8K`, adding a procedure to "Gaat
over". The task also has `onderdeel_van: [PROJ-DP5C]`, which `edit_taak` renders
only as the incoming side of `bestaat_uit`.

Unlike BUG-HOB9BR (same message, candidate-page truncation), this is permanent,
not transient: the advice to reload cannot work, because the entity GET returns
the same untyped key every time. The field is simply unsavable.

## Reproduction

1. Have an entity with two relations, where the form renders only one as an
outgoing picker.
2. Open the edit form and change the rendered relation.
3. Autosave aborts with the toast; nothing is written.

## Root cause

`DynamicForm.vue:590` seeds `relations.value` from the entity GET, which returns
EVERY relation the entity has. `pickerTypes` is filled only by a RENDERED
`RelationPicker` (`updateRelationTypes`). A relation with no outgoing field on
the form therefore has ids but no types, `reshapeLegacyToModern` returns null on
the first untyped id, and the whole relations payload is dropped — including the
edit the user made.

Both save paths had it: `handleSubmit` and `buildAutoSaveRelationsBody`. The
only filter between `relations.value` and the payload excluded `cards`
relations, so everything else rode along.

## Fix

New `frontend/src/components/forms/ownedRelations.ts` decides which keys a form
may put on the wire; both save paths filter through it.

Ownership follows the field's DIRECTION, because only an outgoing field keeps
state in `relations.value`. An incoming picker loads its own edges and delivers
them through `pendingCardChanges` under the inverse body key, so the inverse key
from the GET is a stale untyped duplicate — owning it was the bug.

Two guards kept in place:

- `link_as: to` prefills a relation that may have no field on the form and
registers its own type, so it is kept explicitly via `alsoKeep` — otherwise the
post-condition check at the submit site turns a working create into a hard
error.
- Ownership reads `allFields`, not the affordance-filtered `fields`, so a
server-hidden relation is still owned rather than silently dropped. This also
fixes a latent throw in `buildRelationsPatch` for a hidden incoming relation
with pending changes.

Beyond the abort, this stops a form asserting a full-replace value for edges it
never showed the user.

The toast now names the offending relation instead of giving advice that cannot
help.

## Verification

- 18 new unit tests; mutating the helper back to the old behaviour fails the
two integration tests, so they pin the fix rather than passing vacuously.
- Full frontend suite green (196 files / 3177 tests), typecheck and lint clean.
- Driven in a real browser against atlas on the postgres backend: no toast,
chip count 11 -> 12, PATCH carries `gaat_over` with 12 typed identifiers and no
`onderdeel_van`, which the server preserved.
