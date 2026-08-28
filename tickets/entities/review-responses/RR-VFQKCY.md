---
id: RR-VFQKCY
type: review-response
title: revertField restores the server baseline, not the pre-change value — declining can rewind an unrelated accepted edit
finding: |-
    The plan (BUG-FB0LN8 'Agreed fix' step 5) delegates the decline-path restore to useAutoSave.revertField(). But revertField (useAutoSave.ts:545-549) restores from lastSeenServer[property], which is written only by recordServerSnapshot / mergeServerResponse — the last value the SERVER confirmed, not the value the field held before this change.

    Failure scenario: inkooproute starts 'aanbesteding'. User changes it to 'europees' (hides nothing, no dialog, autosaves). Before that PATCH lands (800ms debounce + round-trip), the user changes it to 'onderhands', gets the clear dialog, and declines. revertField restores lastSeenServer['inkooproute'], still 'aanbesteding' — so the decline silently rewinds the user's already-accepted 'europees' edit. The user declined 'clear the deadlines' and lost 'set route to europees'.

    Fix: capture an explicit pre-change snapshot of the trigger field at gate time and restore from that, rather than delegating to revertField's server-baseline semantics.

    Related side effect: applyServerProperty (DynamicForm.vue:1233) fires void loadEntity(true) when the reverted property is in transitions.value — a full entity refetch as a consequence of a revert, which also stomps formData via :347 while retained values are outstanding.
severity: significant
resolution: |-
    Implemented. revertField is NOT used for the decline path. DynamicForm tracks lastEdit (set in updateField before formData is mutated), and captureTriggerSnapshot reads from that, so a decline restores exactly the value the edit replaced — not autoSave's lastSeenServer baseline, which can be older than an intermediate edit the user already accepted.

    Pinned end-to-end: 'clear_when_hidden:confirm keeps the value when declined' asserts BOTH that the conditional field keeps its value AND that the trigger (done) reverts to true. This test fails against the pre-fix component.
status: addressed
---
