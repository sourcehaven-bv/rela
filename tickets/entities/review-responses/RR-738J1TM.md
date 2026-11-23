---
id: RR-738J1TM
type: review-response
title: Free-text cap was silent
finding: A free-text search with more than maxSearchResults hits served the first 1000 with total 1000 and no signal that rows were dropped.
severity: significant
resolution: The search probes one hit past the cap and reports meta.capped; SearchView shows a capped notice. Pinned by TestSearchPage_FreeTextCapIsReported.
status: addressed
---
