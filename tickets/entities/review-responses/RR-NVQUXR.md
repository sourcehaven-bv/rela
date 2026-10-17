---
id: RR-NVQUXR
type: review-response
title: Resync replays channel subscriptions without a topic
finding: allChannels selects subscriptions whose last answer was 403/404, so an overflow fallback can send live:changed to subscribers of hidden channels.
severity: minor
resolution: Round selection requires a topic; TestLive_ResyncSkipsTopiclessSubscriptions pins that topicless subscriptions are neither replayed nor framed on overflow.
status: addressed
---
