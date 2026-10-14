---
id: RR-V97BTD
type: review-response
title: Schema loaded more than once per list operation
finding: gateAppListQuery loaded a.Meta() and a.Cfg() separately, and the list path loaded the schema again for narrowing and pushdown.
severity: nit
resolution: listPage, scopedSortedEntities and storePosition load a.State() once and pass it to gateAppListQuery and resolveListNarrowing; listNarrowing keeps it for pushdownPlan. nav_status passes its existing schema.
status: addressed
---
