---
id: TKT-87VSDE
type: ticket
title: 'Required file properties: decide when a required attachment is due, and enforce it'
kind: enhancement
priority: low
effort: m
status: wont-fix
---

## Closed: the premise was wrong

This ticket asked "when is a required attachment *due*?", presenting three
candidate semantics as an open decision. Investigation showed the decision was
already made and implemented — the ticket was written from a misreading of
`internal/metamodel/validation.go`.

**`required` is a soft rule, server-side, for every property type.**
`ValidationErrorRequired` is classified soft (`validation.go:132`, `IsSoft()`)
and mapped to the `required_property_unset` warning
(`entitymanager/validation.go:44`). Verified end to end against a scratch
project declaring `evidence: {type: file, required: true}`:

```
$ rela create report -P title="Test report"
WARNING: required_property_unset at /properties/evidence: This field is required
✓ Created report REP-001          ← exit 0

$ rela analyze properties
✗ Found 1 property errors:
  REP-001 (report): This field is required
```

So the entity is created with a warning and `analyze properties` keeps flagging
it until a file lands. That is the "due eventually" option this ticket proposed
building — it already exists, and it is not file-specific.

The rationale is sound and worth recording: FS-backed content can be edited
outside rela, so the server cannot treat `required` as a hard guarantee. The
data-entry form, which *does* control its own input, layers a client-side gate
on top (`DynamicForm.vue` `validate()`, pinned by the e2e test "form blocks POST
when required fields are missing"). Soft server rule + client affordance, with
`analyze properties` as the backstop. On a networked backend like postgres the
gate approximates a real invariant closely, but it remains an affordance, not a
security boundary.

The two other options this ticket floated were both worse:

- *Hard-fail at create* would make a required file property permanently
unsatisfiable, since the file can only arrive after the entity exists.
- *Refuse the declaration at schema load* would delete a working feature.

## What was actually broken

One real defect, and it was narrower than anything described here: staged files
live outside `formData`, which is where the client-side gate looks, so a
required `file` property blocked Create with no way to satisfy it.

Tracked and fixed as **BUG-L1DHC5**.
