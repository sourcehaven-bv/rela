---
id: RR-PZHJNN
type: review-response
title: 'clear_when_hidden: confirm cannot be built correctly on the current form architecture — scope reduced to no|yes'
finding: |-
    Three separate fixes to the interactive confirm path each passed their tests and each then failed in manual use:

    1. The revert restored formData but not the server — updateField stages the autosave before the dialog exists.
    2. The pre-edit value lived in a single mutable slot (lastEdit) that a second watcher pass could consume, so the revert silently became a no-op.
    3. An 800ms debounce raced the dialog: thinking too long committed the change.

    Root cause is structural, not a sequence of coding errors. updateField mutates formData AND arms the autosave in one step, so there is no 'proposed but not committed' state. Anything needing to intervene must reconstruct prior state after the fact, from state scattered across lastEdit, autosave's pending map, and retained. That reconstruction is where every bug lived.

    Notably: neither adversarial code review, nor 1462 unit tests, nor 236 e2e tests caught any of these. The user found all three in minutes of clicking.
severity: critical
resolution: |-
    Scope reduced. 'confirm' is REMOVED from the accepted enum — not left as an accepted-but-degraded value. A config using it now fails validation loudly at author time, which is honest; a value that silently behaves like something other than its name is the exact trap that caused this bug.

    Shipped: no (default) and yes. Both are decided synchronously from state the caller already holds, so there is no window in which form and server can disagree. This is the actual data-loss fix and it is fully verified.

    Removed with it: the async gate, resolveHide, gateBusy, withSuppression/isSuppressed, lastEdit, captureTriggerSnapshot, loadGeneration, and the confirm-dialog wiring. useHiddenFieldPolicy went from 196 to 105 lines and is now a pure synchronous query; applyHidePolicy went from ~70 lines to 12.

    Kept: useAutoSave.cancelPendingField (tested, unused in prod) — the 'drop a staged write without side effects' primitive the refactor's reject path needs.

    Follow-up: TKT-7S5735 (propose/commit refactor — policy manager + write queue), with confirm as its first consumer and validating use case.
status: addressed
---
