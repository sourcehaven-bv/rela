---
id: RR-TRD40V
type: review-response
title: Double post through the shared useConfirm promise
finding: 'onDecide checks `deciding || submitting` only in step 1, before `await confirm(...)`, and sets `deciding` only in step 4. useConfirm is a singleton whose concurrent calls share one in-flight promise (frontend/src/composables/useConfirm.ts header: ''both callers receive the same user decision''). Two activations before the modal renders (double click, key repeat, header and row in quick succession) both await the same confirm, both get true, both pass the step-3 shownDecisions re-check, and both POST. The form path has the same gap across `await nextTick()`. Fix: take the lock before the first await (set deciding at the top, release it on cancel), or re-check `deciding`/`submitting` after every await. Add the case to AC 5''s test (two clicks on Abandon, confirm once, one POST).'
severity: significant
resolution: onDecide now takes the lock and pending key before any await (confirm, nextTick, typing settle) and releases it in finally, with a "two clicks on Abandon, confirm once, one POST" test in AC 7.
status: addressed
---
