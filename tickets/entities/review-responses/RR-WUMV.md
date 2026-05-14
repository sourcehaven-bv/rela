---
id: RR-WUMV
type: review-response
title: 'Cranky #1: Manager.UpdateEntity mutates caller''s entity in place'
finding: Manager.UpdateEntity mutates the supplied *entity.Entity when automation sets properties. Pre-flip wsEntityManager cloned first. Contract change is invisible.
severity: significant
resolution: Documented the mutation contract on Manager.UpdateEntity godoc + on Manager.CreateEntity (the latter explains the supplied e is consumed and the freshly-built entity returned via CreateResult.Entity).
status: addressed
---
