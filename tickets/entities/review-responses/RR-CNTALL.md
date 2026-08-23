---
id: RR-CNTALL
type: review-response
title: CountEntities began materializing every match even in the default world
finding: "fsstore.CountEntities and memstore.CountEntities changed from an integer counter to building a []entityMeta / []*entity.Entity of every matching row before taking its length, in order to hand the set to keepPrimes/worldKeep. For the DEFAULT world those helpers return their input unchanged, so a whole-store count now allocated a slice of every entity in the store for no benefit — on the common path for any project that never declares a pointer, which the WorldScope docs require to stay allocation-free (IsDefaultWorld exists precisely so backends can take the historical fast path)."
severity: minor
status: addressed
resolution: "Restored the allocation-free counter for q.World.IsDefaultWorld() on both backends, keeping the buffered path only for a real world. Comment states why the fast path exists."
---

**Finding (code review, TKT-WAV8XP PR-B).**

Performance-only; no behavioral difference. Worth fixing because the
zero-cost-for-pointerless-projects property is an explicit design commitment in
`store.WorldScope`'s documentation, not an incidental optimization.
