---
id: RR-8JVD41
type: review-response
title: The create label is dropped
finding: '§1 and §9: `create.label` compiles into `ChannelType.Create.Label` (Gate.Label), and action labels are Gate.Label too, but ChannelTypeInfo omits the create label, so the "create" button for `many` types gets a fixed SPA string. Fix: add `create_label` to ChannelTypeInfo.'
severity: nit
resolution: The descriptor and wire carry create_label from create.label (R29).
status: addressed
---
