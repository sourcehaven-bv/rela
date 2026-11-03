---
id: RR-UMCUN7
type: review-response
title: reminderCounters recomputed for every entry under the store lock
finding: realtime/session.go:512 calls reminderCounters(s, cur, ch) inside the per-entry loop, up to 8 times under the lock for one post. Compute it once before the loop.
severity: nit
resolution: reminderCounters is computed once before the entry loop, and the decider returns early when the state has no entries for the type.
status: addressed
---
