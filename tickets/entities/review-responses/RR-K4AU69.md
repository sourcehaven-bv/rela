---
id: RR-K4AU69
type: review-response
title: DynamicForm validationErrors field-error branch is dead against the real backend
finding: The migrated branch reading ApiError.validationErrors maps errors[] to per-field messages, but the backend's write-path validation (writeV1Error) never populates errors[] — only the analyze endpoint uses the field-error struct. The branch never executes (pre-existing; the old duck-typed branch was equally dead).
severity: minor
reason: Deferred to the A7 follow-up (surfacing autosave validation warnings in form fields), where per-field error/warning surfacing gets designed end-to-end — deciding there whether the backend should populate errors[] on writes or the branch should be dropped.
status: deferred
---
