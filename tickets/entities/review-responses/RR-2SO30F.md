---
id: RR-2SO30F
type: review-response
title: Safety replay puts every inbox pair into one round and overflows liveRoundCap/liveRoundBudget at modest scale
finding: 'live.go:665 runs the safety round on one tick, and live_inbox.go selectsInbox (f.safety) selects every inbox subscription that has a wake set. Each principal holds two pairs (summary and page). The memo merges their computation but not the pairs, so 129 or more principals with both pages open exceed liveRoundCap (256, live.go:48). On fs the 10 s liveRoundBudget is hit far sooner: at about 0.4 s per Inbox with liveParallel 4, about 100 principals use up the budget. The pairs that do not fit get a fallback frame (live.go:739 and :760). Each such client then refetches GET /api/v1/_inbox or /_inbox/summary outside the hub with no memo, and the pair goes to pending to be replayed at the next grid tick. Result: a periodic refetch storm every 12 to 14 s that grows with user count, and badge refetches with no change. AC 6 measures only 20 principals, so it does not catch this. Fix: spread the safety replay over the safety period, for example a slice of the subscriptions per grid tick keyed by a stable hash of the pair. Alternatively, exclude safety-only selections from fallback frames: keep them pending instead of calling markChanged.'
severity: minor
resolution: The safety replay is staggered into fixed slices by an FNV-1a hash of the principal (7 on databases, 3 on fs), one slice per grid tick. TestLive_InboxSafetyStaggered with 70 principals.
status: addressed
---
