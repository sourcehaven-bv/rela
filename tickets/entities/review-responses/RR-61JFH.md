---
id: RR-61JFH
type: review-response
title: 'Concurrency: warnings reflect post-merge state — add explicit AC'
finding: 'Risk Assessment notes ''Concurrent two writers — sanity check'' but doesn''t engage with warning angle. handleV1UpdateEntity takes writeMu.Lock(), so writes serialize. But response warnings reflect validator''s view of entity as-merged for THIS PATCH, not on-disk state at response-construction time. Plan''s framing ''warnings show what''s wrong with post-write entity'' needs precision: ''post-write entity as observed by this request''s validator pass'' = merged in-memory state. Recommendation: add concurrency AC: two interleaved PATCHes on same entity, one creating soft condition (title=''''), second touching status. Both responses include warnings from merged state at validation time. Cheap test using two goroutines and writeMu already in place. From design-review F10.'
severity: minor
resolution: 'AC28 added: two interleaved PATCHes on same entity, writeMu serializes, both responses include warnings reflecting their respective merged in-memory state at validation time (not on-disk state at response-construction). Test uses two goroutines.'
status: addressed
---
