---
id: RR-CB08Y
type: review-response
title: ConfirmModal missing focus trap — Tab escapes the dialog
finding: ConfirmModal has role=dialog and aria-modal=true but no focus trap. Tab can escape the dialog into the background page. All four custom modals in the app share this gap. Acceptable for this ticket given two buttons is a small surface, but file a follow-up for a shared useFocusTrap composable.
severity: minor
reason: Agreed this is a genuine accessibility gap but out of scope for TKT-AYU8, which is a targeted UX ticket. Filed as follow-up TKT-X4P99 covering a shared useFocusTrap composable for all six data-entry modals, not just ConfirmModal — solving it once for ConfirmModal would leave five other modals still broken, so it makes sense to tackle the class of problem together.
status: deferred
---
