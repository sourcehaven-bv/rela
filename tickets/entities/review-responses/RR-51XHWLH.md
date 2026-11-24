---
id: RR-51XHWLH
type: review-response
title: No backend conformance or audit assertion for removal
finding: Tests run on memstore only and use audit.Nop
severity: minor
reason: Removal reuses UpdateEntityIf whole-row replacement which PatchEntity MetaUnset already relies on; the audit summary diffs old and new rows
status: wont-fix
---
