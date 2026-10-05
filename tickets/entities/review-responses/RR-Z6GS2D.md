---
id: RR-Z6GS2D
type: review-response
title: Sync splice and push hide zero-face reads from the guard
finding: spliceEntity read GetEntityState(key, ent.Face) where ent.Face is always zero, a zero-face read the guard cannot see; on a faced type ApplyEntity would then write a zero-face row.
severity: minor
reason: A syncFace variable now names the default-world assumption (ruling D8) in splice.go. Refusing or carrying faces is a sync protocol change that belongs with the world work (TKT-7IZHP0 / PR 8), not this read migration.
status: deferred
---
