---
id: RR-X52REQ
type: review-response
title: Reminder messages lose the actions their message type declares
finding: 'Plan line 635: messageActions offers nothing on a reminder, as it does for notices (msgops.go:81-84). That also drops the custom actions and promote_to of the declared type (raw.go:93-94), so an operator who adds an acknowledge action to the reminder type sees it on a person''s post and never on a reminder. Plan change: either refuse at load a post.type with actions or promote_to ("a reminder message offers no actions"), or refuse only edit, delete and remove and keep offeredActions. Add the matching row to AC 2 or AC 14.'
severity: minor
resolution: 'Addressed: actions and promote_to on a reminder type are kept, not refused. messageActions already offers declared actions on system posts such as trigger posts; only edit and delete are withheld. Rows in AC 1 and AC 14.'
status: addressed
---
