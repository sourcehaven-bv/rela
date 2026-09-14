---
id: TKT-QF41FL
type: ticket
title: Narrow the relations-autosave abort to the affected field, not the whole form
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`reshapeLegacyToModern` returns a single nullable record for the whole relation
set, so one unresolvable ID makes `buildAutoSaveRelationsBody` (and the submit
path in `DynamicForm.vue`) abort **every** relation field on the form. The user
sees "Some related entities have unknown types; relation changes were not
saved", no PATCH is sent at all, the chips stay on screen, and the edit is lost
on reload.

BUG-HOB9BR made the trigger much rarer but not impossible: it still fires at
`listAllEntities`'s 50-page cap, and on any future consumer that populates
`pickerTypes` partially. The blast radius is unchanged.

## Approach

Mechanical: have `reshapeLegacyToModern` return per-relation results rather than
one nullable record, and let the caller drop only the affected keys while the
rest of the form saves normally.

## Why this was not done in BUG-HOB9BR

It is a behaviour change to the shared save path used by every relation field on
every form, so it needs its own test matrix. Bundling it into a bug fix would
have made the regression surface much larger than the defect. Filed rather than
left to memory because it is a known whole-form-abort on a shared save path.

## Related

- BUG-HOB9BR — made this trigger rarer, did not remove it
- RR-A049JA — the review finding that asked for this ticket
