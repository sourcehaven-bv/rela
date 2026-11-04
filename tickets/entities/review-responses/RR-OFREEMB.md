---
id: RR-OFREEMB
type: review-response
title: Tab bar can still appear late
finding: The channel schema loads lazily, triggered only by ChannelsPanel (channelSchema.ts:182-185); the tab bar was placed outside the viewData condition.
severity: minor
resolution: Deferred to TKT-B75I7LH with the tab design; the planned fix is recorded there.
reason: 'Applies only to placement: tab, which moved to TKT-B75I7LH (approved scope change: this ticket ships above_relations only).'
status: deferred
---
