---
id: RR-S1A1LN
type: review-response
title: Discard-confirm dialog rendered underneath the inline modal
finding: ConfirmModal and InlineCreateFormModal both use .modal-overlay at z-index 1000 (App.vue:507) and both Teleport to body. ConfirmModal mounts at app root, so the later-teleported inline modal paints on top at equal z-index. requestClose() on a dirty form therefore awaited a confirm the user could neither see nor click — the modal appeared frozen on Escape. Same hazard class as the useConfirm singleton the ticket set out to avoid, reached via stacking instead of promise-sharing.
severity: critical
resolution: The inline modal's scoped .modal-overlay is lowered to z-index 900, below ConfirmModal's 1000, so the discard prompt always paints above the dialog that raised it.
status: addressed
---
