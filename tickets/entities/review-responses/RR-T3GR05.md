---
id: RR-T3GR05
type: review-response
title: matchRelationFilterMany failed open on ne when the store errored
finding: Both error paths returned a partial or empty verdict map; the caller evaluates operator==ne && !matched[id], so an empty map made every row match the exclusion filter and the request still returned 200. A backend fault silently widened a narrowing filter (api_v1.go). Same swallow shape in the view section resolvers.
severity: critical
resolution: matchRelationFilterMany returns (map, error); applyRelationFilters wraps it in errListLoad so the request fails loud. The section resolvers (relationColumnTargets, visibleTitles) log a Warn naming the relation and count before truncating, since a view renders what it could load but never quietly.
status: addressed
---
