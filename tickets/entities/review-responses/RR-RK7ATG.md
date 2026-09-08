---
id: RR-RK7ATG
type: review-response
title: Singleton confirm dialog shares one promise across concurrent triggers — a second clear is approved by a dialog that never named it
finding: |-
    useConfirm (frontend/src/composables/useConfirm.ts:118-122) documents its own concurrency semantics verbatim: 'if (pendingPromise) { // Concurrent call: return the in-flight promise. Both callers observe the same user decision. return pendingPromise }'.

    Failure scenario: user toggles a, dialog opens naming 'Inschrijfdeadline'. The form stays interactive — nothing in the plan disables it. User toggles d, which would clear Vragenronde_deadline. The second confirm() call returns the FIRST promise and shows the FIRST dialog's text. User clicks Yes meaning 'clear Inschrijfdeadline'. Both call sites receive true, so Vragenronde_deadline is cleared without ever being named in any dialog — precisely the silent-data-loss class this ticket exists to eliminate.

    This directly violates the plan's step 6 promise of 'one batched dialog naming each field and value'. Resolution must either serialize trigger evaluation behind an explicit queue, or hard-block form input while a clear-confirm is pending. The plan must state which.
severity: minor
resolution: |-
    Downgraded critical -> minor. The finding's premise ('the form stays interactive — nothing in the plan disables it') was asserted without checking the markup, and is wrong.

    ConfirmModal (frontend/src/components/ui/ConfirmModal.vue:91) renders a full-screen .modal-overlay backdrop. While a dialog is open the rest of the form is not mouse-reachable, so the 'user toggles a second trigger field during the async gap' scenario cannot occur by face.

    Residual risk is narrow: keyboard focus could in principle reach a control behind the overlay if focus is not trapped, and a same-field re-toggle could re-enter the gate. Both are closed by a single re-entry guard — hold a pendingGate flag and have the gate return early (ignoring the change) while a dialog is outstanding.

    No queue, no serialization machinery, no promise-sharing workaround needed. Reuse the existing useConfirm/ConfirmModal pair as-is; add the one guard and pin it with a test asserting a second trigger during an open dialog is ignored rather than silently inheriting the first answer.
status: addressed
---
