---
id: RR-R8U30U
type: review-response
title: Strip is a live region inserted with its content
finding: RlChannelComposer.vue:552 puts role=status on an element that v-if inserts already populated. Many screen readers do not announce content that arrives together with its live region. Keep the region mounted and swap its content, or rely on the focus move and aria-describedby instead of role=status.
severity: nit
resolution: "addressed\tRemoved role=status from the strip. The focus move and the send button's aria-describedby announce it. The ComposeMode story now asserts that the role is absent."
status: addressed
---
