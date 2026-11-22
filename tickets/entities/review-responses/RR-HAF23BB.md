---
id: RR-HAF23BB
type: review-response
title: "Documents and flow runs read twice after an action"
finding: "The live-update events also make these panels read again, so an action can cause two reads."
severity: minor
reason: "The events are not guaranteed (no subscription, another world, a dropped connection), so the revision is the reliable signal; a second read of a small list is cheap and the panels keep their content while it runs."
status: wont-fix
---
