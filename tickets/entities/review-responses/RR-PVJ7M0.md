---
id: RR-PVJ7M0
type: review-response
title: RlChannelMessage exposes focus() that nothing calls
finding: RlChannelMessage.vue:88-91 adds `root` and `defineExpose({ focus })`. RlChannel.focusRow (RlChannel.vue, new defineExpose) finds the row with querySelector('[data-seq=...]') and focuses the element directly, as jumpTo does. The exposed method and the template ref are dead API on a library component. Remove them, or make focusRow call them.
severity: nit
resolution: Removed the unused root ref and defineExpose({ focus }) from RlChannelMessage.
status: addressed
---
