---
id: RR-AH5GN6
type: review-response
title: The seq watch fires only when the channel changes
finding: 'The plan sets linkedSeq "in the same watch as linked". That watch is on route.query.channel alone (ChannelsSection.vue:145-152). A click on a second mention in the same channel changes only ?seq=, so the watch does not fire and nothing is revealed. AC 10 "seq change reveals the new target" cannot pass. Fix: watch channel and seq together, or watch seq on its own. Re-applying the link when only seq changed must not toggle or re-fetch the channel.'
severity: significant
resolution: ADDRESSED ChannelsSection watches query.channel and query.seq together; a seq-only change sets the pending reveal and selects the channel directly, with no toggle or re-fetch; deep-link case added (AC 10, Approach 5).
status: addressed
---
