---
id: RR-1C1KA5
type: review-response
title: Differential harness misses divergence classes
finding: No headless entities, string-only lists, one real, world ranking checked against the same store's ListEntities, about 17% of queries falling back to naive via quo"te, and no depth above DepthCap.
severity: significant
resolution: The reference is now graphquerynaive over an identically seeded memstore, so world ranking is checked independently. The unsafe name is picked 1 in 40 per property instead of 1 in 9. Depths reach past DepthCap. Headless entities, non-string lists and extra reals expose known divergences in both SQL backends and are filed with their fixes as BUG-LCHDSR.
status: addressed
---
