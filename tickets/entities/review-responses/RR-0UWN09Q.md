---
id: RR-0UWN09Q
type: review-response
title: Composer keeps an agent-only type after an Other answer is posted
finding: 'After select() of an offer: false type and a successful post, reset() left the type selected and in the picker, so the next message went out as that type.'
severity: significant
resolution: reset(posted) ends the request when the posted type is the requested one; ending the reply ends it too. Stories OfferFalseSelectPosted and OfferFalseSelectReplyEnds.
status: addressed
---
