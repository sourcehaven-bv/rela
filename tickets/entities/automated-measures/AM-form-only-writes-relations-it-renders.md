---
id: AM-form-only-writes-relations-it-renders
type: automated-measure
title: Form save omits relation keys the form does not render as an outgoing field
description: 'Mounts the edit form over an entity carrying both a rendered outgoing relation and an unrendered one, edits the rendered one, and asserts the PATCH goes out with no error toast and without the unrendered key. Also pins that every prefill channel (rel.* params, link_as: to, Duplicate) survives the filter, since dropping one would be a silent no-write.'
kind: test
location: frontend/src/components/forms/DynamicForm.foreignrelations.test.ts
status: active
---

Mounts the edit form over an entity carrying both a rendered outgoing relation
and an unrendered one (the inverse key of an incoming field), edits the rendered
one, and asserts the PATCH goes out with no error toast and without the
unrendered key. Mutating ownedRelationKeys back to the pre-fix pass-through
fails these tests, so they pin the behaviour rather than passing vacuously.
Paired with unit coverage of the ownership decision in ownedRelations.test.ts.
