---
id: RR-DWKEHB
type: review-response
title: Deleting a channel refetches the list twice
finding: For a delete, onLifecycle emits `changed` and then `gone` (OpenChannel.vue:343-344), and ChannelsSection answers each with list.refetch() (lines 85 and 153). Emit only `gone` when the channel is null.
severity: nit
resolution: A lifecycle delete now emits only gone, so the list is read once.
status: addressed
---
