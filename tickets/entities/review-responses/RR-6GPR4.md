---
id: RR-6GPR4
type: review-response
title: Manual-ID edit-mode behavior for DynamicForm not fully specified
finding: 'The plan says ''Manual-ID field is read-only in edit mode'' but DynamicForm currently renders nothing for ID in either create or edit. The plan needs to specify: (a) in edit mode, show the ID as a disabled/read-only text display (for visual consistency with create), OR (b) don''t render it at all. Just saying ''read-only'' is ambiguous. Also: DynamicForm is instantiated by `/form/:id/:entityId?`—if entityId is present (edit mode) AND the type is manual, we should NOT show an editable input because that would imply rename, which is a separate feature (FEAT-016).'
severity: significant
resolution: 'Plan now specifies: create mode = editable input; edit mode = read-only `<div class="id-display">` with helper text pointing to rename feature. Rename behavior explicitly out of scope (deferred to FEAT-016). AC3 updated with matching component test.'
status: addressed
---
