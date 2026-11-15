---
id: RR-6P0YNH4
type: review-response
title: Id read cost claim false on fs and mem
finding: graphquerynaive.MatchingFaces read every row of the type and dropped non-candidates afterwards, so on fsstore and memstore the id read still scanned the type.
severity: significant
resolution: graphquerynaive.MatchingFaces now passes the candidate ids into its ListEntities read (both the plain and the world-branch path)\; a world ranks each id on its own, so no prime changes.
status: addressed
---
