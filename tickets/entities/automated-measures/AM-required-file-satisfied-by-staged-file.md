---
id: AM-required-file-satisfied-by-staged-file
type: automated-measure
title: 'Regression tests: a staged file satisfies a required file property'
description: 'Unit and e2e tests asserting that a staged (not-yet-uploaded) file satisfies a `required: true` file property in the create form: submit is blocked with the field flagged while none is staged, the standing error clears the moment one is, and the create then proceeds with the attachment landing. Guards the divergence between the two readers of ''does this property have a value?'' — formData and stagedFiles.'
kind: test
location: frontend/src/components/forms/DynamicForm.attachments.test.ts, e2e/tests/attachments-create.spec.ts
status: active
---

## Measure

Four tests that fail against the pre-fix code, covering the divergence between
the two places that answer "does this property have a value?".

**Unit** — `frontend/src/components/forms/DynamicForm.attachments.test.ts`,
against a type declaring `document: {type: file, required: true}`:

- submit with no file → no create, and the field is flagged
- a staged file satisfies the requirement → create and upload both proceed
- the standing error clears the moment a file is staged

**E2E** — `e2e/tests/attachments-create.spec.ts`, against a real server:

- a blocked Create issues no POST (excluding the `?dry_run=true` affordance
probe) and shows the required error; staging a file then lets the create
through, and the attachment reads back from the API

The unit reproduction was `created: 0, uploaded: 0` before the fix and `1`/`1`
after, so these are genuine regression tests rather than tests written to pass.

## Fixture note

The required file property lives on its own `signoff` entity type in
`e2e/tests/fixtures.ts`, deliberately **not** on the shared `bug` type. A
required property on a shared type makes every other test's entity emit a
`required_property_unset` warning — the first attempt did exactly that and broke
an unrelated assertion.

## What it does not cover

The systemic cause in why5 — that "has a value" is derived ad hoc from
`formData` at each call site rather than through one accessor. These tests pin
the two readers that exist today; a third value source added later would not be
caught. A single `hasValueFor(property)` accessor is the structural fix, noted
on BUG-L1DHC5 and not built here.
