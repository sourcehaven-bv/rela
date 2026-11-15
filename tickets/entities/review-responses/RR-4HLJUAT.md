---
id: RR-4HLJUAT
type: review-response
title: scanKey relied on an unstable sort by property only
finding: Config.lower sorted pushed predicates by property only with an unstable sort, so equal sets could give different keys.
severity: nit
resolution: Sort by property, then value, with cmp.Or.
status: addressed
---
