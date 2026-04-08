---
id: RR-TMEM
type: review-response
title: indexEntity two separate Loads can yield meta+idx from different epochs
finding: indexEntity does idx := w.searchIdx.Load() then meta := w.meta.Load(). If Reload slides in between, idx is old but meta is new. entityToSearchDocument(entity, meta) then indexes a new-shape doc into the old index. Low impact but solved by the same single-snapshot fix as the meta/automation tearing.
severity: minor
resolution: Fixed by the workspaceState bundling (see RR-6G1K). indexEntity now does `s := w.state.Load()` once, then uses `s.meta` and `s.searchIdx` from the same snapshot. No more two-call epoch mismatch.
status: addressed
---
