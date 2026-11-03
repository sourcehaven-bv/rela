---
id: RR-BGI06O
type: review-response
title: Cooldown, post cap and dirty-shard state are per process, so with alternating lock holders D11 bounds multiply and stale seen data can overwrite fresh data
finding: 'internal/realtime/trigger.go:91-94 (lastPost, failures, dirty), 890-897 (postOne cooldown), 624-636 (round.shard prefers t.dirty over the KV), 932-935 (save rewrites dirty shards). Because the lock moves between processes at every run (see the significant finding above), consecutive rounds for one entity can run in different processes. Scenario 1: P1 posts enter for TKT-1 at t0. A change at t0+1s is handled by P2, whose lastPost is empty, so it posts at once. The one-post-per-minute cooldown (D11) becomes N posts per minute for N processes, and MaxSystemPostsPerRound becomes N times 64 per tick window. Scenario 2: a shard save fails in P1 and P1 keeps the shard in t.dirty. P2 later updates the same shard key in the KV. P1''s next round reads its stale in-memory copy instead of the KV, posts again what P2 already posted, and save() writes the stale copy over P2''s newer data. That loses P2''s entries and causes more reposts at the next reconcile. Fix: the held leader lock fixes both. Until then, clear lastPost, failures and dirty whenever the worker passes from not leading to leading (the takeover reconcile already reloads the KV).'
severity: minor
resolution: With a lifetime leader the cooldown, cap and unsaved shard state belong to one process. Reset clears them on takeover and on loss, so stale seen data is never written over a newer leader's.
status: addressed
---
