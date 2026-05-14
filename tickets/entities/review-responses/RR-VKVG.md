---
id: RR-VKVG
type: review-response
title: 'Cranky #8: Manual-ID check duplicated between Manager.CreateEntity and createCore'
finding: Both Manager.CreateEntity (line 134) and createCore (line 49) call IsManualID() and produce customIDNotAllowedError on rejection. Mild duplication.
severity: minor
reason: Manager.CreateEntity checks IsManualID as part of pre-createCore duplicate-detection (so the 'wrong id_type' error wins over coincidental ID collision). createCore independently validates as defense-in-depth (cascadeHost.CreateEntity calls it directly). Two checks at two layers serve different concerns.
status: wont-fix
---
