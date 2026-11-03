---
id: RR-CU5WM5
type: review-response
title: Marks share the post budget and the 64-bucket cap with the identity
finding: 'opRead is charged under IdentityRate (300 per minute for all channel writes of user plus org). Because the tool is ignored, a browser marking 60 times a minute uses up a fifth of the agent''s post budget for the same user. Each channel marked also adds a per-channel bucket, and MaxRateBucketsPerIdentity is 64. An agent clearing 50 unread channels and then posting in a few of them within one second hits 429 on buckets, not on rate. Fix: charge marks to their own aggregate bucket, not IdentityRate (a mark is idempotent and private). Or state the sharing in the docs and the MCP mark_read description. Test: 64 marks in one second do not block a post.'
severity: minor
resolution: 'Marks use a separate limiter: ReadRate per reader and channel, ReadIdentityRate aggregate, own bucket caps. TestMarkRead_OwnLimiter: 64 marks then a post succeeds. Documented in mark_read.'
status: addressed
---
