---
id: RR-YHPQJA
type: review-response
title: revealSeq is not tied to the linked channel and is never used up
finding: 'applyLink sets linked to null once it has opened the channel (ChannelsSection.vue:131-132), but nothing says when linkedSeq is cleared. While ?seq= stays in the URL, revealSeq would go to whichever OpenChannel is open (ChannelsSection.vue:227-236, keyed by channel). Picking another channel in the list would try to reveal the same seq in the wrong channel. Closing and reopening the linked channel would jump to the old target again. Fix: pass revealSeq only while open.key equals the key the link opened. Clear it after one goTo, or drop seq from the URL with router.replace.'
severity: significant
resolution: ADDRESSED Pending reveal is {key, seq}, passed only to the OpenChannel of that key, used once, cleared on revealed and on a switch to another channel, and seq is dropped from the URL with router.replace; cases for another channel, reopening and URL cleanup (AC 10).
status: addressed
---
