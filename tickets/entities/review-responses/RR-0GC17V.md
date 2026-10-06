---
id: RR-0GC17V
type: review-response
title: Form-made link can be duplicated
finding: If the create form already sets the same relation (to another anchor), the menu adds a second parent. Under a max-incoming cap the link fails and the toast says link it by hand although the row is linked.
severity: minor
resolution: Not changed in this bug.
reason: The tab New button has the same behaviour today. Detecting a form-made link needs the created entity's relations, which the create response does not carry. Worth a separate ticket if it shows up in practice.
status: deferred
---
