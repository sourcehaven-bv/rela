---
id: RR-YBVK0GF
type: review-response
title: No backend test of a HasOutbound endpoint with nested HasInbound
finding: The role-relation shape of the run gate was only exercised on memstore.
severity: significant
resolution: New storetest subtest HasOutbound_endpoint_with_nested_HasInbound covers GraphQuery, MatchingFaces and GraphCount; it passes on mem, fs, sqlite and pg. The randomized graphdiff generator is unchanged.
status: addressed
---
