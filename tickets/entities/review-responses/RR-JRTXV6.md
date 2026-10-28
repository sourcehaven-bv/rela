---
id: RR-JRTXV6
type: review-response
title: Addressed wake set is added to every subscription, but only the MCP watch drains it
finding: 'The plan has markAddressed add targets to `addressed` on every Subscription, says the set "shares max with changed (overflow is a resync)", and says "the live hub ignores it in this slice". Take() clears only `changed` and `reads` (local.go:170). The live hub''s SubscribeReads subscription therefore accumulates targets until it overflows, then enters resync. Depending on the implementation, it either keeps flipping to full resyncs or grows without bound. Fix: copy the existing opt-in pattern. Add `SubscribeAddressed()` (as SubscribeReads sets allReaders), let markAddressed write only to opted-in subscriptions, and have Take''s resync branch clear `addressed` too. Add a local_test row: a plain Subscribe never sees addressed hints and never resyncs from them.'
severity: significant
resolution: Added SubscribeAddressed opt-in; markAddressed writes targets only to opted-in subs; Take's resync clears addressed; TestLocal_AddressedOptIn added.
status: addressed
---
