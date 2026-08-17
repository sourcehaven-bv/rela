---
id: RR-MYHDH0
type: review-response
title: Confirm dialog may name a redacted field the principal cannot inspect
finding: fieldByProperty (DynamicForm.vue:223) builds from allFields, not the redaction-filtered fields, so policyFor can resolve a clear_when_hidden for a redacted property. Harmless today since applyHidePolicy only reads formData, where redacted props are absent. Under confirm it becomes a dialog about a value the user cannot see. Per root CLAUDE.md naming the field is fine - this is a UX issue, not a security one, and the plan should say which it means.
severity: minor
resolution: Security Considerations now distinguishes the UX concern from a security one and records why fieldByProperty resolves policy for redacted properties.
status: addressed
---
