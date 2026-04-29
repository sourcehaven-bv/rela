---
id: RR-W5DGH
type: review-response
title: Searcher errors silently degrade to no results
finding: runFreeTextSearch returns nil on iterator error. freeTextIDsForType then builds an empty map and returns (emptyMap, true). The handler intersects with that empty map and the user sees an empty list with no indication anything broke. AC9 'empty result state shows No matches' becomes a lie when the index is broken.
severity: critical
resolution: Refactored runFreeTextSearch into runFreeTextSearchE that returns (entities, error). freeTextIDsForType now returns (freeTextIDsForTypeResult, error) and the list handler converts errors to HTTP 500 instead of rendering an empty list. Added TestV1ListEntitiesSearchQuery sub-test 'searcher error surfaces as 500' to lock the behavior.
status: addressed
---
