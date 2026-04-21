---
id: RR-87ICD
type: review-response
title: Read-only ID display only shown for manual-ID types in edit mode
finding: DynamicForm.vue:70-72 only displays the entity ID in edit mode for manual-ID types via showReadOnlyID. For a non-manual type in edit mode, there's no read-only display of the ID at all. Inconsistent treatment.
severity: minor
reason: Pre-existing behavior, not introduced by this PR. The rationale is that for manual-ID types the user is the source of the ID so showing it back disambiguates which entity is being edited; for short/sequential types the ID is incidental and the title/name property serves that role. Changing this to always-show would conflict with form layouts in production projects that rely on the current behavior. Out of scope for the prefix-picker / manual-ID-input ticket.
status: wont-fix
---
