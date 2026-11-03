---
id: RR-VO5F0Q
type: review-response
title: The store event channel is drained on the same goroutine as long rounds, so events are routinely dropped
finding: 'internal/appbuild/realtime_worker.go:343-385. `case ev := <-events` shares the select with the tick, the reconcile timer, the minute lifecycle and the hourly retention, all run inline. A reconcile or baseline over a large type, or the retention sweep, can take seconds. During that time the 1024-slot subscription (realtime_triggers.go:13) fills up, and the store drops events without any signal. Those changes then wait for the next reconcile, up to about 16.5 min, so a trigger fires minutes late under exactly the load where it matters. Fix: drain events on a dedicated goroutine that only calls Triggers.Note (Note is already mutex-safe and non-blocking). Alternatively, have the store report a lag/overflow so the engine can mark the type full. Also handle a closed channel (`ev, ok := <-events; if !ok {...}`) so a store Close that races the worker does not busy-spin.'
severity: significant
resolution: Store events are drained on their own goroutine, which notes changes only while the process leads. Long rounds no longer block the subscription.
status: addressed
---
