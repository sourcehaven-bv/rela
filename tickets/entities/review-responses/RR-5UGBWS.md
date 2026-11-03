---
id: RR-5UGBWS
type: review-response
title: 'Empty channels cannot be followed: after_rev must be >0 and next_after_rev is omitted'
finding: 'Section 3.2 requires after_rev to be a positive integer, but the newest page of a channel with no messages has channel.Rev 0, and the omitempty Cursors drop next_after_rev. channels.Page treats 0 as unset, so an agent has no cursor to watch from. The CLI watch default (head rev at start) quietly drops into newest-page mode. Fix: accept after_rev=0 as an explicit cursor (presence, not value) and always emit next_after_rev on newest and after_rev pages.'
severity: significant
resolution: after_rev is a presence-based cursor, so 0 is valid. next_after_rev is always emitted on newest and after_rev pages.
status: addressed
---
