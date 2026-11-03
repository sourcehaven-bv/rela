---
id: RR-0NPRJU
type: review-response
title: Auto-opened single channel never sees new channels
finding: 'In single mode (one row and !canCreate, ChannelsSection.vue:59) the channel opens automatically, so channelOpen is true and the list subscription is dropped (`live: () => !channelOpen.value`, line 49) for as long as the page is shown. The user cannot close the channel to bring the list back. A second channel created by another user who holds the create role therefore never appears until a lifecycle change or a navigation, while docs/realtime.md and GUIDE-realtime.md promise that ''a new channel shows up without a reload''. Fix: keep the list subscription in single mode (it costs one extra subscription, only when there is exactly one channel), and add a ChannelsPanel test where a list frame in single mode brings a second channel and the list appears.'
severity: significant
resolution: A directly opened single channel keeps the list subscription, and when a second channel appears the list shows with the open channel still selected, pinned by a ChannelsPanel test.
status: addressed
---
