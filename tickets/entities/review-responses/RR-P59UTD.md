---
id: RR-P59UTD
type: review-response
title: Directional oracle vacuity — reject-everything backend would pass
finding: Unconditional return on rejected creates means a backend rejecting all writes passes the collision fuzz targets trivially.
severity: significant
resolution: 'Deduped into createEntityOrSkip with a documented vacuity anchor: storetest.RunAll''s Entity conformance suite asserts plain creates succeed and every backend runs both — over-rejection fails loudly there; the fuzz targets chase collision/round-trip divergence only.'
status: addressed
---
