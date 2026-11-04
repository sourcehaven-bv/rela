---
id: RR-NYD6CG9
type: review-response
title: ChannelsPanel key ignores types
finding: The key is entityType/entityId (ChannelsPanel.vue:40); when a reload moves a type to another block the section keeps state for a type it no longer holds.
severity: minor
resolution: 'Plan: the key includes the sorted types.'
status: addressed
---
