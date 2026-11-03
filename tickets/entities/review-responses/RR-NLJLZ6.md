---
id: RR-NLJLZ6
type: review-response
title: Graph-write subtests race the changefeed pump with a fixed three-tick wait
finding: 'internal/dataentry/live_busy_test.go:297 and :613 write to the store, then step(t, 3) and expect the full round. The store event reaches liveHub.run through the changefeed Bus pump goroutine (internal/changefeed/changefeed.go:94), so the manual stepper can finish three ticks (a few ms) before feed.Wake fires. Then no later tick is sent and requireChanged times out. It passed under CPU stress here, and existing inbox tests share the pattern, but it is a timing assumption, not a guarantee. Fix: add a helper that steps one tick at a time until a condition holds (the frame arrived, or the hub''s full-round counter moved), with a wall-clock deadline. Alternatively, wait for the feed subscription to report the write before the first step.'
severity: minor
resolution: 'Fixed: busyLiveEnv.stepUntil steps one tick at a time until a condition holds (board replayed, board pending, wake cleared), with a 10 s deadline, and fails before the first safety tick. It replaces the fixed three-tick waits.'
status: addressed
---
