---
id: RR-Y81RSR
type: review-response
title: A shard save that keeps failing is retried on every round forever
finding: 'internal/realtime/trigger.go:939-956. After TriggerRetries the error is logged once, but the shard stays in dirty and save() re-Puts it on every later round. That differs from section 13 ("retried for 3 rounds, then logged and marked seen"). It is harmless while the KV is down, but it is unbounded retry traffic. Fix: either document that seen-set saves retry indefinitely (in memory) or back off by time.'
severity: minor
resolution: A failing shard save now waits with the same doubling delay, capped at one minute, and is logged once after the retry window. It stays in memory while the process leads so a made post is not repeated.
status: addressed
---
