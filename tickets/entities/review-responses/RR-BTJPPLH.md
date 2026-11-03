---
id: RR-BTJPPLH
type: review-response
title: Step markup duplicated between the short list and the full list
finding: The bullet, outcome tag or caption and Input required tag are written twice in RlChannelSession.vue.
severity: minor
reason: The two lists differ in element, role text and aria-hidden rules; a shared component would need eight props for about ten template lines. Both copies sit next to each other in one file, and the stories pin both.
status: wont-fix
---
