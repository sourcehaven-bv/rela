---
id: RR-RC2DKKN
type: review-response
title: CTE timing side channel not documented
finding: In a non-default world the index scan reads faces before visCond drops the hidden ones, so latency grows slightly with hidden matches, as the default world already does.
severity: minor
resolution: 'Fixed: documented in buildSearchSQL''s comment.'
status: addressed
---
