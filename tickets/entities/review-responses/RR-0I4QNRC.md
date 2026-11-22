---
id: RR-0I4QNRC
type: review-response
title: Step form ignores a field's default
finding: FlowStepForm.vue shows nothing for an empty subject value where the entity form would show the field default.
severity: minor
resolution: An editable field the subject holds no value for starts at the form field default ($user resolved) or else the schema property default. Complete sends it. Tested in FlowStepForm.test.ts.
status: addressed
---
