---
id: RR-5ZYL5G
type: review-response
title: Fixed DOM id in ChannelTextDialog
finding: ChannelTextDialog.vue:302 and :329 use the literal id `channel-text-dialog` to tie the submit button to the form. Two instances (create in ChannelsSection, remove in OpenChannel) would collide. Use useId() for the form id.
severity: nit
resolution: ChannelTextDialog takes its form id from useId().
status: addressed
---
