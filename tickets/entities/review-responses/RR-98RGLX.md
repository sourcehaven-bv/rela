---
id: RR-98RGLX
type: review-response
title: Reconnect cost understated and synchronized across tabs
finding: Risks says each reconnect "resumes from the last event id and re-PUTs the live-view subscriptions". A reconnect costs more than that. setLiveConnection resets accepted to an empty map, so after the re-PUT the onChange of every registered key fires and every subscribed GET is refetched. On the server, handlePut runs a baseline replay for every subscription. All tabs of one browser session usually carry the same proxy-issued token. They reach exp at the same moment and reconnect together. With 5-minute tokens, each user gets a full refetch burst every 5 minutes. The design does not need to change, but Risks should record this cost. If the load matters, end each stream at a small random time before exp (for example 0 to 5 percent of the remaining lifetime, capped) to spread the reconnects. This addresses load across tabs, which is a different concern from the fixed margin the plan rejected.
severity: minor
resolution: 'Plan updated: Risks records the client refetch, the server baseline replay per subscription and the synchronized tab reconnect, bounded by the per-identity cap; no jitter, with the reason.'
status: addressed
---
