---
id: RR-ZZS3KX
type: review-response
title: A failed reminder post can make the loop spin at the 1s floor
finding: 'Plan lines 600-603: when validateProps or whenHolds fails, the failure is logged and counted in Failed, but the plan does not say whether the channel is held back or its slot advanced. If neither happens, the hint stays at or before now, the write sets More (timers.go:606-607), and the channel is retried every second indefinitely. The plan says this "cannot happen for a config that loaded". That holds only while the metamodel enum values behind props cannot change without a realtime reload. Plan change: on a post failure call holdBack, the same path as a failed write (timers.go:593-597), and add a test with a props failure injected.'
severity: minor
resolution: 'Addressed: a failing validateProps or whenHolds calls holdBack and counts Failed, with no More for that channel. TestTimers_RemindPostFails injects a props failure.'
status: addressed
---
