---
id: RR-QP1F3V
type: review-response
title: Reader-scoped Subscription path has no production caller
finding: 'internal/realtime/local.go:102,123,224. SubscribeAs, the Subscription.reader field and the `!s.allReaders && s.reader == reader` branch of markFor are only used in reads_test.go. Production subscribers are the live hub (SubscribeReads), the MCP watch (Waker.Subscribe) and the CLI watch (svc.Subscribe), all with the zero reader. The plan''s per-connection SubscribeAs was replaced by one hub-wide SubscribeReads that tags marks by reader. That deviation is sound: liveSub.reader plus roundFilter.readTopics scopes replays per reader, and a replay renders the subscriber''s own principal in any case. But the old mechanism was left in place as dead weight, and TestLocal_MarkForScopesReader now tests code no transport uses. Fix: delete SubscribeAs and the reader field, make markFor feed only allReaders subscribers, and rewrite the test against SubscribeReads/TakeReads.'
severity: minor
resolution: Removed SubscribeAs and Subscription.reader; scoping tests rewritten against SubscribeReads/TakeReads
status: addressed
---
