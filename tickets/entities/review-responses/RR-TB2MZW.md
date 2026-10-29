---
id: RR-TB2MZW
type: review-response
title: ChannelTextKey comment now heads the wrong keys
finding: useMessages.ts:141 inserts '// Session decisions' + 'decisions' between sessionPausedArchived and sessionShowAll/sessionHideAll. The comment now visually claims the two stepper keys. Move 'decisions' after sessionHideAll.
severity: nit
resolution: Moved the 'decisions' ChannelTextKey after sessionHideAll under its own comment.
status: addressed
---
