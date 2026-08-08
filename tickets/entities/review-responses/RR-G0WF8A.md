---
id: RR-G0WF8A
type: review-response
title: Blanket watcher suppression swallowed coalesced user edits; concurrent trigger asked for an unperformable revert
finding: |-
    Two related defects in the revert machinery.

    1. isSuppressed() gated the ENTIRE watcher body. Vue coalesces a revert mutation and a same-flush user mutation into ONE watcher invocation, which then observes isSuppressed() === true — so the user's change is dropped entirely: a field that just hid is never retained or error-cleared, and a field that just revealed is never restored.

    2. resolveHide returned {clear: [], revert: true} for a concurrent trigger while a dialog was open. The caller reverts from captureTriggerSnapshot(), a SINGLE slot belonging to the trigger that opened the dialog. After the lastEdit-consumption fix the second trigger gets {} — a silent no-op revert; before it, it would have reverted the WRONG property. revert:true is unperformable with a single-slot snapshot.
severity: significant
resolution: |-
    1. Suppression is now scoped per-property: withSuppression(fn, properties) records which keys the revert re-hid, isSuppressed(prop) checks membership, and the watcher skips only those properties rather than the whole pass. The reveal-restore loop and the RR-U9ERK error-clear now run normally for unrelated properties in the same flush. Pinned by 'suppresses only the properties the revert touched'.

    2. A concurrent trigger now returns revert:false and is simply ignored. That is the honest outcome: the second branch hides and keeps its value (retention still holds it, so revealing restores it), rather than claiming an undo the caller cannot perform. Test updated to assert revert:false and documented in the composable.
status: addressed
---
