---
id: RR-O7L9ON
type: review-response
title: Notice is not announced when it arrives with its causing message
finding: 'The plan keeps the stepper out of live regions because "the notice is announced by RlChannel''s polite region" (Approach 4 and 6). But the store appends the causing message and the notice in one append (internal/channels/session.go:143), so another reader''s follow brings both in one frame. RlChannel.announce (frontend/packages/rela-components/src/components/channel/RlChannel.vue:206-217) then sees added.length == 2, `only` is undefined, and it reads "2 new messages" with no notice text. Other readers never hear that the session moved. AC 13 only appends a lone notice, so it passes anyway. Plan change: announce() reports the non-notice messages as today and appends the text of each added notice (for example "1 new message from Ann. Session moved from Detected to Contained."). Add an AC 13 row that appends a message and its notice in one update and checks that the notice text is in the polite region.'
severity: significant
resolution: 'Plan updated: announce() reports non-notice messages as today and appends each added notice''s text (Approach 7); AC 14 AnnouncesMessageAndNotice appends a message and its notice in one update and checks the polite region.'
status: addressed
---
