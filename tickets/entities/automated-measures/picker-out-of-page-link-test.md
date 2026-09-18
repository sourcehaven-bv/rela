---
id: picker-out-of-page-link-test
type: automated-measure
title: 'Test: relation picker resolves types for links outside the candidate page'
description: Unit coverage asserting a RelationPicker emits a type for a pre-existing link whose target is absent from the capped candidate page, renders it as a selected chip, preserves it across a later selection, and degrades safely when the resolving lookup fails. Verified to fail before the BUG-LSCDJK fix.
kind: test
location: frontend/src/components/forms/RelationPicker.test.ts (describe "pre-existing value outside the candidate page (BUG-LSCDJK)")
status: active
---

Locks in the fix for BUG-LSCDJK.

The fixture is the load-bearing part: the candidate page deliberately omits the
linked entity, which is what a project with more than 100 entities of a target
type produces in practice. A fixture where every link is also a candidate cannot
observe this class of bug at all, which is why it survived.

Four cases:

- **emits a type for a pre-existing id that is not in the candidate page** —
the direct cause. Without a type, `reshapeLegacyToModern` returns null and
DynamicForm drops the relations payload.
- **renders a pre-existing out-of-page link as a selected chip** — the
secondary symptom, since `selectedEntities` also filtered `candidates`, so the
user could not see or remove the link.
- **keeps the out-of-page type after the user adds another peer** —
multi-select, guarding against a rebuild that drops the resolved type at the
moment of editing.
- **still emits types for in-page ids when the relations lookup fails** — the
resolve is best-effort; a transient failure must not reintroduce the save-abort
it exists to remove.

Verified to bind to the fix: reverting only `buildOutgoingTypes` to its old
candidate-scan re-fails cases 1 and 3.
