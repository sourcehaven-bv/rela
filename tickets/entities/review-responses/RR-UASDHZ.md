---
id: RR-UASDHZ
type: review-response
title: Outputs runner runs synchronously in the leader loop and can stall timers, triggers and the leader lock check for minutes
finding: 'internal/appbuild/realtime_worker.go:513-516 calls w.runOutputs(ctx) inline in lead()''s select loop. One round can take up to 16 marks x 4 outputs x promoteTimeout (30 s) plus mark writes, which is over 30 minutes in the worst case. During that time the timer round (session after: timers), trigger rounds and the verify/check of the leader lock do not run. Another process can take the lock while this one keeps writing. Fix: run the round in its own goroutine with a single-flight guard and a round deadline (for example under realtimeLifecycleInterval), or bound the round''s total time and leave the rest for More.'
severity: significant
resolution: The outputs round runs in its own goroutine, one at a time, bounded by a 1-minute timeout, ending with the lead; TestWorker_OutputsRoundBesideTheLoop.
status: addressed
---
