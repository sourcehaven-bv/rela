---
id: RR-9SC0KR
type: review-response
title: useMarkRead clears inFlight on a target switch while the old request is still pending
finding: 'frontend/src/composables/useMarkRead.ts:110 (target watch) and :98 (finally). If the target changes during a mark, the watch sets inFlight=false and the new target may send a mark. Then the old request''s finally sets inFlight=false again, even though the new request is still in flight. A second check can then send a concurrent mark. The server is monotonic, so the only harm is a spent read-limit token, which can turn into a 429. Fix: keep inFlight per generation (for example, store the generation that owns the in-flight request and clear it only when mine === generation).'
severity: nit
resolution: inFlight records the owning generation so only that request clears it; vitest test fails on the old code
status: addressed
---
