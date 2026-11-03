---
id: RR-LMCQX1
type: review-response
title: Every typing call and every subscriber Take does a full O(entries) sweep of the typing store
finding: 'internal/realtime/typingstore.go:64 (touch calls sweepLocked), :170 sweepLocked, :190 nextDeadline, and local.go:180 expire(), which Subscription.Take (local.go:118) runs on every wake of every subscriber. With the caps at 32 x 4096 entries, each refresh (one per typist every 3s) and each Take walks the whole map under t.mu, and scheduleExpiry walks it again. Cheap at today''s numbers, quadratic at the caps the code advertises. Fix: keep the earliest deadline cached in the store (recompute only when the entry holding it is removed or refreshed) and return early from sweepLocked and nextDeadline when now is before it; or keep a deadline min-heap.'
severity: minor
resolution: The store keeps a min-heap of deadlines; touch, admits and live read only their channel; nextDeadline is O(1); TestTypingStore_StaysInItsChannel pins it with a visit counter.
status: addressed
---
