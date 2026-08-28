---
id: RR-R5VL59
type: review-response
title: aria-disabled swap silently changes Cancel-button behaviour in modals, risking double-submit and focus regressions
finding: |-
    The plan states pending controls use aria-disabled rather than native disabled, and that the handler is suppressed in JS. Correct in general, but the plan does not scope WHICH buttons this applies to, and the blanket rule breaks two existing behaviours.

    1. ConfirmModal disables BOTH buttons while busy (:108 cancel, :116 confirm). Cancel is disabled deliberately — useConfirm.ts:89-96 sets busy=true, awaits onConfirm, and clears busy on throw so the user can retry. If Cancel becomes aria-disabled and merely stops emitting, the user can still Tab to and activate it, and there is no defined behaviour for cancelling an in-flight confirm. Either keep native disabled on Cancel, or define what cancel-during-flight means (abort? ignore? close and let the write land?). The plan defines none of these.

    2. ConfirmModal focuses Cancel on open and restores focus on close (a documented WAI-ARIA dialog requirement, :55-60). Changing Cancel's disabled semantics interacts with that focus management; it must be re-verified, not assumed.

    3. The double-submit risk is understated. The plan calls it "a second POST ... not a privilege one", but for a DELETE confirm a second activation is a second destructive request. Suppressing the handler must be verified by test (AC9 covers the click path only — add keyboard activation, since aria-disabled buttons remain keyboard-reachable, which is the entire point of using it).

    Required: state explicitly that aria-disabled applies to the PRIMARY action button only; Cancel/secondary buttons keep native disabled or stay enabled with defined semantics.
severity: significant
resolution: 'Plan Approach §3a added, scoping aria-disabled to the PRIMARY action button only; secondary/Cancel buttons keep native disabled. Rationale recorded: aria-disabled keeps a control focusable AND activatable, so every such button needs defined in-flight semantics. ''Ignore the second activation'' is correct for a primary action but undefined for Cancel, since useConfirm.ts:89-96 holds busy across the awaited onConfirm and cancelling an in-flight confirm has no defined meaning (abort/ignore/close-and-let-it-land). Defining that is explicitly out of scope. ConfirmModal.vue:55-60''s focus-Cancel-on-open behaviour is therefore untouched. AC9 extended to require handler suppression for keyboard activation (Enter/Space) as well as click, with a matching negative test, because keyboard reachability is the entire point of aria-disabled. The double-submit risk is restated in Security Considerations as a correctness/idempotence issue (a second activation on a destructive confirm is a second DELETE), not merely cosmetic.'
status: addressed
---
