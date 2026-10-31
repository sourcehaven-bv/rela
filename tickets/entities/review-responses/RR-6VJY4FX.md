---
id: RR-6VJY4FX
type: review-response
title: Only the first property run had a live event stream
finding: useEvents and useLiveSubscription keep module state across unmounts, so runs after the first sent no subscription PUT and saw no events; the event source had no scheduler.
severity: significant
resolution: 'cleanup() closes the shared stream and calls resetLiveSubscriptions; fakeEventSource gets the scheduler. Measured: all 100 runs send a subscription PUT.'
status: addressed
---
