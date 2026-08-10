---
id: conditional-field-reveal-roundtrip-test
type: automated-measure
title: 'Test pin: a visible_when field survives a hide/reveal round-trip in edit mode (value intact, field re-renders)'
description: Asserts that hiding and re-showing a visible_when field in edit mode leaves its stored value intact and re-renders the field. Fails against the pre-fix prune + formData-presence render gate that caused BUG-FB0LN8.
kind: test
location: frontend/src/components/forms/DynamicForm.test.ts (+ e2e/tests/)
status: proposed
---

Regression pin for BUG-FB0LN8.

A DynamicForm unit test (and an e2e counterpart) that, in **edit** mode on an
entity with a populated conditional property:

1. Flips the governing property so the conditional field hides.
2. Flips it back.
3. Asserts the conditional field **re-renders**, and that its stored value is
still present — both in the form state and in the payload sent to the API.

The test must fail against the pre-fix code, where `pruneWizardHidden` /
`scheduleUnset` drops the key and the `f.property in formData` render gate then
keeps the field permanently hidden.

Complementary assertion worth pinning in the same suite: renderability in edit
mode is decided from the entity type's **declared properties**, so a declared
property that is merely *unset* still renders, while a property withheld by ACL
(absent from the metamodel-visible set / marked in `_fields`) does not.
