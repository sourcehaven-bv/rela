---
id: RR-LFXFGW
type: review-response
title: Post-await clear/revert acted on form state the server had since replaced — destroys a value the user never approved
finding: |-
    applyHidePolicy awaits a confirm dialog, but formData can be replaced underneath it: loadEntity(true) fires from applyServerProperty (a committed state-machine move) and onAttachmentChanged, and autosave's applyServerProperty merges server values directly.

    Scenario A (clear): deadline='OLD'. User toggles the branch, dialog names 'Deadline: OLD'. Before they answer, a status transition triggers loadEntity(true) and the server returns deadline='FRESH' (a concurrent editor's write). User clicks Clear, approving the loss of OLD — FRESH is unset instead. This is BUG-FB0LN8's own failure mode (silent destruction of a value the user never consented to lose) re-entering through the async gap the fix introduced.

    Scenario B (revert): user edits mode a->b, dialog opens, a merge lands mode='c' from another editor, user clicks Cancel. The revert stomps mode back to the stale 'a', bypassing updateField so no autosave is scheduled — the form shows 'a' while the server holds 'c'.
severity: critical
resolution: |-
    Fixed with a load-generation fence plus per-value verification.

    1. New loadGeneration ref, bumped in loadEntity (before formData is replaced) and in autosave's applyServerProperty. applyHidePolicy captures it before the await and returns early if it changed — a decision about state that no longer exists is abandoned rather than applied.

    2. The clear loop additionally captures the exact values the user was shown (values Map) and skips any property whose current value differs, so a merge landing mid-dialog cannot be destroyed by a 'yes' that did not cover it.

    The fence also closes the gap in the releaseAll fix: retention is dropped by loadEntity, and the in-flight gate now knows to abandon.
status: addressed
---
