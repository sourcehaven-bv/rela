---
id: RR-S7M4O5A
type: review-response
title: Stale comments after the switch to LowerCondition
finding: 'cranky: QueryScopeLower doc (empty=true), LowerScope doc, the pgstore EXPLAIN test still using LowerScope, and the viewconditions.go claim that any drift is a compile error.'
severity: minor
resolution: 'Addressed: QueryScopeLower and LowerScope docs updated, the EXPLAIN test lowers through LowerCondition, and the viewconditions.go comment says the optional Lower/EntityType methods are caught by the dataentry differential and budget tests.'
status: addressed
---
