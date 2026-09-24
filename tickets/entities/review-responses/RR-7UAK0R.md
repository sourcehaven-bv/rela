---
id: RR-7UAK0R
type: review-response
title: World ranking vs filtering diverges between pg and naive
finding: pgstore applied Props/Narrowing before the world's DISTINCT ON rank; naive ranks first. Porting pg's shape would port the bug.
severity: critical
resolution: Filed BUG-2SKLD3 and fixed pg first (rank, then Props/Narrowing; Any/FaceIn/relations trim candidates before the rank). The sqlite builder mirrors the fixed shape.
status: addressed
---
