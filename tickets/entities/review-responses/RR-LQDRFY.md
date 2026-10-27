---
id: RR-LQDRFY
type: review-response
title: The SPA ignores 429, so a lost mark leaves a stale badge
finding: 'The client marks "at most once per second" and the server bucket is 1 per second per reader and channel, with no burst. Network jitter puts a request at 0.98 s after the previous one, which gets 429. The plan says a rate-limited call is ignored and the SPA re-checks only on visibilitychange. The marker therefore stays behind until the next message arrives, and the channel the user is looking at shows as unread in their other tabs and lists. Fix: retry with a trailing-edge debounce after Retry-After (or keep the newest seq pending and flush it when the bucket refills), and/or give ReadRate a burst of 2. Test: a 429 followed by a successful retry with the newest seq.'
severity: significant
resolution: 'ReadRate is 2 per 2 s (burst 2); SPA keeps the newest seq pending on 429 and sends it once after Retry-After. useMarkRead test: 429 then one retry with the newest seq.'
status: addressed
---
