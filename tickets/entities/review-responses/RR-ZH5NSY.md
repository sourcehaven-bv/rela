---
id: RR-ZH5NSY
type: review-response
title: AC5 warn-condition is underspecified; negating HasUnconfiguredScan() is wrong
finding: 'The plan states the new WARN fires "when a scan command IS configured for any file property" but does not name an accessor for that predicate, and none exists. metamodel.AttachmentPolicy exposes HasUnconfiguredScan() (the inverse) and ScanCommandFor(prop). Negating HasUnconfiguredScan() is INCORRECT: it returns true when at least one file property has NO command, so a metamodel with two file properties — one with a scan_cmd, one with `scan: off` — yields HasUnconfiguredScan()==false while a scan genuinely IS configured, suppressing the warning exactly when it is needed. The correct predicate is: exists a file property where ScanCommandFor(prop) returns non-nil, which honours both the `scan: off` opt-out and the per-property override. This requires a new AttachmentPolicy.HasConfiguredScan() method in internal/metamodel/attachments.go — a file the plan''s ''Files to modify'' list omits.'
severity: significant
resolution: 'Plan updated: the condition now uses a new AttachmentPolicy.HasConfiguredScan() defined as "exists a file property where ScanCommandFor(prop) is non-nil", which reuses the existing resolver and therefore honours `scan: off` and per-property overrides. internal/metamodel/attachments.go added to the Files to modify list. Added a regression edge case (one property with scan_cmd + one with scan: off + unusable sandbox => WARN MUST fire), which is precisely the case the negated predicate got wrong.'
status: addressed
---
