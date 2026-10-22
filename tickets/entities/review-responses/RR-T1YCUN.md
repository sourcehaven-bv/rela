---
id: RR-T1YCUN
type: review-response
title: '"Caused by message 7" names a number users never see'
finding: 'The UI shows no seq anywhere on a message row (RlChannelMessage.vue). The causing message is always the one directly before the notice, because both are written in one append (internal/channels/session.go:143). The button text means nothing to users and the jump moves about one row. Plan change, the simplest one: drop the cause button and channelSessionCause. If the button stays, label it from the held message ("Caused by Ann''s Contained message", built from author and type label in noticeView) and fall back to "Show the causing message" when that message is not held.'
severity: minor
resolution: 'Plan updated: cause button and channelSessionCause dropped; notice view has no cause; the causing message is the row above (Approach 7, AC 11 asserts no jump control).'
status: addressed
---
