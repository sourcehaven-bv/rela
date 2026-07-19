---
id: RR-SG2I1G
type: review-response
title: Create-lock formData adoption over-reaches to all read-only fields
finding: DynamicForm.refreshStagedAffordances adopts candidate.properties[name] into formData for EVERY writable===false field, not just machine fields, despite the comment scoping it to state-machine fields. On create this is usually a harmless self-copy, but a non-machine policy-read-only field with a server-transformed value would silently overwrite the user's typed value. Either scope precisely or fix the comment to admit it covers all read-only fields. Recommend extracting to a pure helper adoptLockedFields(fields, properties, formData) and unit-testing it (closes the untested-adoption gap too).
severity: minor
resolution: Extracted the adoption to a pure exported helper adoptLockedFieldValues(fields, properties, formData) in stagedEntity.ts; the doc now correctly states it covers ALL read-only fields (machine-locked or policy-read-only) and explains why that scoping is safe (read-only values are server-owned). Added 4 unit tests in stagedEntity.test.ts covering adopt/skip/untouched/no-op cases.
status: addressed
---
