---
id: RR-ZFRJMF
type: review-response
title: Declining the clear-confirm reverted the form but not the server — the trigger's autosave was already staged
finding: |-
    Found by manual demo testing, not by any automated check.

    updateField() stages an autosave for the edited field IMMEDIATELY (scheduleFieldSave at the end of the function) — before the activeProperties watcher runs, and long before the confirm dialog resolves. The decline path restored formData from its snapshot but never touched that staged write.

    Result: choosing Cancel restored the dropdown in the UI while the server kept the NEW value. Reloading showed the change had been saved anyway. The plan's claim that 'nothing is staged into pending before the dialog resolves' was true for the fields being cleared but never true for the TRIGGER field itself.

    Worse, the form and server silently diverged — the user sees 'aanbesteding', the server holds 'onderhands'.
severity: critical
resolution: |-
    Added useAutoSave.cancelPendingField(property): drops a not-yet-sent write and its debounce timer WITHOUT revertField's form-state restore (the caller owns the restore and knows the exact value; revertField would apply its own, possibly staler, lastSeenServer baseline). Returns whether anything was cancelled.

    The decline path now cancels the trigger's pending write; if it had already fired, it re-saves the snapshot value instead (routing through scheduleUnset vs scheduleFieldSave via isClearedForType so a cleared trigger unsets correctly).

    Pinned by strengthening the existing e2e decline test: it now waits out the debounce and re-asserts server state. Verified the test genuinely catches this — removing the fix makes it fail, restoring it makes it pass. Plus three useAutoSave unit tests for cancelPendingField (cancels before firing, does not touch form state, reports false once fired).
status: addressed
---
