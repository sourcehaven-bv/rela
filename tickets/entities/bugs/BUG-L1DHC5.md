---
id: BUG-L1DHC5
type: bug
title: 'A required file property makes the create form unsubmittable: Save silently does nothing'
description: 'On a create form with a `file` property declared required: true, picking a file and pressing Create does nothing — no entity, no upload, no visible error. validate() reads formData, but staged files deliberately live outside formData (TKT-7K3BJF), so the required check can never be satisfied. Regression from #1479.'
priority: high
effort: s
why1: Pressing Create does nothing because handleSubmit's `if (!validate(...)) return` gate (DynamicForm.vue:1032) rejects the form.
why2: validate() (DynamicForm.vue:833) decides required-ness by reading `formData.value[propName]`, and a staged file is not there.
why3: Staged files are deliberately held in a separate `stagedFiles` ref, because a File in formData would be POSTed as the property's value — the server stamps that property itself once the bytes land.
why4: TKT-7K3BJF introduced that separate staging store but did not update the one other place that answers 'does this property have a value?', so the two views of 'is this field filled' diverged.
why5: The systemic cause is that 'has a value' is derived ad hoc from formData at each call site rather than through a single accessor, so adding a second value source silently breaks every reader that was not updated by hand. No in-tree schema declares a required file property, so no test exercised the combination and both reviews missed it.
prevention: 'Fix routes the staged-file check through the same required rule rather than adding a parallel one, and adds a regression test asserting a staged file satisfies `required`. Longer-term: a single `hasValueFor(property)` accessor would make a new value source impossible to miss — noted on the ticket, not built here.'
status: done
---

## Symptom

On a create form with a `file` property declared `required: true`, the user
picks a file, presses **Create**, and **nothing happens**. No entity is created,
no upload is attempted, and no error is shown against the file field.

Reproduced with a component test (`created: 0, uploaded: 0`) against a schema
declaring `evidence: {type: file, required: true}`.

## Cause

`validate()` (`frontend/src/components/forms/DynamicForm.vue:833`) decides
required-ness by reading `formData`:

```js
const isRequired = propDef.required || (requiredProps?.has(propName) ?? false)
if (isRequired && (value === undefined || value === null || value === '')) {
```

But staged files deliberately live **outside** `formData` (TKT-7K3BJF): a `File`
in `formData` would be POSTed as the property's value, and the server stamps
that property itself once the bytes land. So staging a file cannot satisfy the
check, and `handleSubmit`'s gate at line 1032 blocks the submit forever.

The error IS written into `errors[propName]` and `FieldShell` renders it
(`.field-error`, `FieldShell.vue:46`) for every field type including file — so
the user does see "This field is required". The defect is that the message is
unactionable: picking a file, which is the only thing it asks for, does not
clear it or unblock the button.

## Regression

Introduced by TKT-7K3BJF (#1479, merged as `ae132dad`). On `develop` only — no
release has been cut, so no user can have hit this. Before that change the file
field was inert in create mode ("Attachment editing unavailable") and the entity
could still be created — the server returned a soft `required_property_unset`
warning and `analyze properties` kept flagging it. Now the field accepts a file
and the form silently refuses to submit, which is the worse of the two.

Neither the design review nor the code review caught it because no in-tree
schema declares a required `file` property, so nothing exercised the
combination.

## Fix

Count a staged file as satisfying `required`, in the place that already owns the
rule:

```js
const hasStaged = (stagedFiles.value[propName]?.length ?? 0) > 0
if (isRequired && !hasStaged && (value === undefined || value === null || value === '')) {
```

This yields the intended behaviour — Save blocked until a file is selected, then
the normal create-then-attach flow — using the same mechanism every other
required field uses.

## Why a client-side gate is the right shape here

`required` is a **soft** server-side rule by design
(`metamodel/validation.go:132` classifies `ValidationErrorRequired` as soft;
`entitymanager/validation.go:44` maps it to the `required_property_unset`
warning), because FS-backed content can be edited outside rela, so the server
cannot treat it as a hard guarantee. `analyze properties` is the backstop.

The data-entry form does control its own input, so gating there is a UX
affordance layered on that soft rule — which is the convention the SPA already
follows for every other property type (`validate()` plus the e2e test "form
blocks POST when required fields are missing"). On the postgres backend, where
out-of-band edits are unlikely, the gate approximates a real invariant closely.
It is deliberately not a security boundary.

## Known residue (accepted)

If the create succeeds and the subsequent upload fails, the entity exists with
the required property unset. That is unavoidable under create-then-attach, and
it degrades to exactly the server's pre-existing soft-warning state, which
`analyze properties` reports. Stated here so it is a decision rather than a
discovery.

Supersedes the premise of TKT-87VSDE.
