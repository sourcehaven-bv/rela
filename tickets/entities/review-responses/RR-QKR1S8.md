---
id: RR-QKR1S8
type: review-response
title: Client-side expiry contradicts D10 and breaks under clock skew
finding: 'D10 claims "no dependence on clock skew", but step 18 filters head.typing_now by expires_at against the browser''s Date.now() through useExpiryClock (frontend/src/components/entity/channels/useExpiryClock.ts:10-33). expires_at is server time and TTL is 6 s. A browser clock 6 s ahead never shows the indicator. One 3-4 s ahead shows it for only 2-3 s of each 3 s refresh cycle, so it flickers. Operator signals have the same flaw, but their TTL can be up to 60 s; at 6 s it decides whether the feature works. Plan change: do not expire typing on the client (see the refresh finding). If client expiry is kept, compute it relative to receipt (receivedAt + (expires_at - server now)), which needs a server "now" in the head, and add a useTyping/channelView test with a skewed Date.now.'
severity: significant
resolution: 'Plan updated: clients do no expiry on typing_now; the SPA shows what the last head says and the server''s expiry mark removes it, so clock skew has no effect (D4, step 17).'
status: addressed
---
