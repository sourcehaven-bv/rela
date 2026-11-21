---
id: BUG-53GP5F5
type: bug
title: Failed flow runs hide their error and blocked steps give no field feedback
description: A failed job step hides its last error from staff and is not flagged in Flow tasks; a retrying step says it waits for no one; a blocked step form gives only a toast; runs are titled by the flow id.
priority: high
effort: m
why1: The job runner logged the last attempt's error but did not keep it on the run; the step form showed every refusal and a blocked step as a toast; the run header's display property was the flow id.
why2: The run view had no field for a job error, and no rule decided who may read one. A write rule's refusal carried only its full message, with no field to show it at, and pattern warnings of a step's write were dropped.
why3: The run page and the step form were built for the happy path of a step done by staff; failure states were left to the server log and generic toasts.
why4: No scenario test walked a failed job and a refused step through the web app as a manager and as a customer.
why5: Flow UX was reviewed per API route rather than per role and state, so what a customer and a manager each see of a failed or blocked step was never written down.
prevention: Go tests pin the job error for managers only, the failed tasks, the rule fields and pattern warnings of a step and the run title; vitest tests pin the inline field feedback, the retrying line and the failed text per flow.
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

A browser test of the harbourmaster signup flow found these gaps in how a flow
run reports trouble.

1. A run whose job step failed says only "failed after 5 failed attempts". The
error of the last attempt goes to the server log, so staff who manage the run
cannot see why it failed. While the job waits for its next attempt, the page
says "Waits for no one. A change, a timer or the system moves it on.", which
does not describe a step that is retrying.
2. A failed run is not flagged anywhere. The Flow tasks page says "Nothing waits
for you", and the run list's `failed` filter does not find a run whose job
failed, because its header keeps the state's status.
3. A step whose next-step condition does not hold shows only a toast. The form
has no required markers and no message at the fields the condition reads, and
the page can scroll away from the form. A schema validation rule that refuses
the write shows only as a toast, and a pattern warning does not show at all.
4. A customer whose step failed reads "Someone who manages this flow has to
retry it", which is staff language.
5. Runs are titled by the lower-case flow id, such as "harbourmaster-signup".

## Acceptance Criteria

- [x] A manager of the run sees the error of the last failed job attempt on the run page, with known secret values removed and the text capped. A reader who does not manage the run never receives it from the API.
- [x] While a job step retries, the run page says which attempt is next, when, and the last error, instead of "Waits for no one".
- [x] The Flow tasks page lists the failed runs the caller manages, with a Retry button.
- [x] The run list's `failed` filter finds runs whose job step failed.
- [x] The step form marks the fields the next step's condition reads as required.
- [x] A write refused by a schema validation rule shows the rule's description at the first field of the form the rule reads, not as a toast.
- [x] A value that fails its type's pattern shows the pattern's message as a warning at its field.
- [x] A blocked step shows a notice in the form, keeps the entries and keeps the form in view.
- [x] A customer reads a friendly text while a job step failed, and a flow can set its own with `failed_message:`.
- [x] A run is titled by the flow's label and the subject's title, or by the flow's `title:` template, instead of the flow id.
- [x] docs/flows.md and the guide describe `title:`, `failed_message:`, the field feedback, the last error and the failed runs.
