---
id: RR-FB1M
type: review-response
title: 'S9: Verdict-flip toast originator must be SectionEditForm (not useAutoSave)'
finding: |
  useAutoSave doesn't know about verdicts; verdict-flip is a server-affordance concept tracked by `_fields`. The toast originator must be SectionEditForm watching the `fields` prop for transitions `writable: true → false`.
severity: significant
status: addressed
resolution: |
  PLAN implementation notes: SectionEditForm has `watch(() => fields.value, (next, prev) => { /* compare writable per-property; for each prop that flipped true→false: 1) call useAutoSave.revertField(prop), 2) call onError("permission changed — your unsaved edits to <label> were discarded", { status: 403 }) */ })`.

  Toast routes through `onError` prop. Unit-testable: mount SectionEditForm, assert `onError` called when `fields` prop changes with a writable flip.
---
