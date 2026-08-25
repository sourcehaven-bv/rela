---
id: RR-VFI1W0
type: review-response
title: PendingButton's label-swap API does not fit ConfirmModal, which appends an ellipsis to a caller-supplied label
finding: |-
    The plan lists ConfirmModal.vue:48-49 as a PendingButton migration site, but the two APIs are incompatible and the plan does not say how to reconcile them.

    ConfirmModal does NOT swap to a distinct pending verb. It computes:

        busyConfirmLabel = computed(() => props.busy ? `${props.confirmLabel}…` : props.confirmLabel)

    The confirmLabel is a caller-supplied prop with many values across the app ("Delete", "Discard", "Overwrite", ...). PendingButton requires an explicit `pendingLabel` and forbids deriving it — for the good reason recorded in the ticket (irregular verbs, translation). But ConfirmModal has no place to get one: the callers pass only confirmLabel, so migrating it would force a new required prop through every useConfirm() call site.

    This is unresolved design, not an implementation detail. Options:
    (a) Leave ConfirmModal on its append-ellipsis behaviour and REMOVE it from the migration list. Its current behaviour is already the least layout-shifting variant (one extra character) and it already uses the U+2026 the ticket standardises on. Document it as a sanctioned exception.
    (b) Give PendingButton an optional `pendingSuffix` mode for callers that genuinely cannot supply a verb, used only by ConfirmModal.
    (c) Add pendingLabel to the useConfirm options and thread it through all callers.

    (a) is recommended — it is zero work and loses nothing. Whichever is chosen, the ticket's "~12 call sites" figure and the migration list must be corrected.
severity: significant
resolution: 'Option (a) adopted. ConfirmModal is removed from the migration list and added to the plan''s OUT-of-scope section as a sanctioned exception (Approach §3b). Confirmed the finding''s premise: confirmLabel is an optional prop defaulting to ''Confirm'' (useConfirm.ts:34,59), so migrating would force a new required pendingLabel through every useConfirm() caller. Its existing append-one-character behaviour is already the least layout-shifting variant and already uses the U+2026 this ticket standardises on. The exception will be documented in frontend/CLAUDE.md so it reads as a decision rather than an oversight. The ticket''s ''~12 call sites'' figure is corrected to 6 in the plan''s Files-to-modify section.'
status: addressed
---
