---
id: RR-81E7VH
type: review-response
title: checkDirty is never reconciled with the reject path
finding: updateField calls checkDirty() after mutating (DynamicForm.vue:1118). Under optimistic apply plus reject-revert, nothing calls it again after the revert, so the form stays dirty with formData identical to originalData and the user gets an unsaved-changes prompt for a change they declined. Trivial to fix, but exactly the passed-its-tests-failed-in-manual-use class this ticket exists to prevent.
severity: significant
resolution: Approach step 6 calls checkDirty on both accept and reject paths; AC2e pins it.
status: addressed
---
