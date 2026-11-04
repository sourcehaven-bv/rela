---
id: RR-T1CKYTL
type: review-response
title: Subscription budget leaves out other subscribers; overflow is silent
finding: The plan counted only channel subscriptions. EntityDetail holds entity and presence (EntityDetail.vue:1099/1110), the inbox badge one, a host list one. A sidebar flyout over EntityView could reach 19 keys; past 16 keys are dropped with only a console warning (useLiveSubscription.ts:59-62).
severity: significant
resolution: 'Non-tab part addressed in the plan: each block holds at most two live keys (its list, or its list and one open channel); full page 7 (entity and presence 2, above_relations and bottom blocks 4, inbox 1), embedded detail 6, worst stream 13 of 16 (flyout over a full page). Budget notes in ChannelsSection, ChannelsPanel and useInboxSummary updated; a Vitest test pins the count. The tab part (active and inactive blocks, summary list) moved to TKT-B75I7LH.'
status: addressed
---
