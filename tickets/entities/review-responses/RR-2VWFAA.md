---
id: RR-2VWFAA
type: review-response
title: SubscribeAddressed sets allAddressed under s.mu, but markAddressed reads it under l.subMu only (data race)
finding: 'internal/realtime/local.go:154 writes s.allAddressed while holding s.mu, after l.Subscribe() has already published s in l.subs. internal/realtime/local.go:261 reads s.allAddressed while holding only l.subMu. The two locks are different, so a concurrent post (markAddressed) and an MCP watch call (newChannelWake -> SubscribeAddressed, tools_watch.go:494) race on the field. The race detector runs in CI. Any test that starts a watch while another goroutine posts can report a DATA RACE and fail intermittently. This copies the existing SubscribeReads/markFor pattern, which has the same race. Fix: set allAddressed and addressed before the subscription is inserted into l.subs (build the Subscription, then insert it under l.subMu), and do the same for allReaders.'
severity: minor
resolution: SubscribeReads and SubscribeAddressed set their flags before registration; immutable afterwards. TestLocal_SubscribeWhileMarking under -race.
status: addressed
---
