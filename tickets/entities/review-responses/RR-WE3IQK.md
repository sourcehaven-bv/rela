---
id: RR-WE3IQK
type: review-response
title: Source lookup runs once per edge before the dedup
finding: The dedup is on the decision tuple, but the type lookup ran for every edge, up to three reads each, contradicting the godoc's handful-of-checks claim for hub entities.
severity: minor
resolution: authorizeCascadeRelations memoizes the resolved type per source id (typeOf map); every face of a family has the same type.
status: addressed
---
