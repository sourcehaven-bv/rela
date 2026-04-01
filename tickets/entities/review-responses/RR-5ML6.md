---
id: RR-5ML6
type: review-response
title: Strikethrough semantic confusion
finding: Current implementation marks item as skipped if ANY strikethrough appears, which could conflate strikethrough-as-formatting with strikethrough-as-skip-marker
severity: minor
reason: The current behavior is intentional and documented. Strikethrough in checklists is conventionally used to indicate skipped/N/A items. Users who use strikethrough for editing within checklist items are using an unusual pattern. The feature description clearly shows the expected pattern.
status: wont-fix
---
