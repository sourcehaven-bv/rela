---
id: RR-R1FVUH
type: review-response
title: Test gaps for the races the plan claims to handle
finding: 'Missing tests: a live frame that lands before the POST response (generation already gen+1 when decidedGen is set); ch.refetch() returning early because a follow is already running (run() dedup sets dirty and resolves at once); the lock release when a 201 moves nothing (see the stuck-lock finding); a close or session move during the in-flight request; the double confirm; a decision row whose cause message gets removed while shown (row buttons disappear and the header stays). channelView tables should also include a type with persist ephemeral and a channel.state ''closed'' head whose _actions has not been refetched yet.'
severity: minor
resolution: Added tests for a live frame before the POST answer, an unfolded refreshHead, a 201 that moves nothing, a close during the request, the double confirm, a removed cause, an ephemeral type and a closed head with stale actions.
status: addressed
---
