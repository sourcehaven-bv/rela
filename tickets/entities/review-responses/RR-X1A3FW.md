---
id: RR-X1A3FW
type: review-response
title: Conformance suite cannot exercise the dataentry serializer (duplicate angle of the serializer-leak critical)
finding: Cases 1-7 assert on search.Hit; none exercises serializeRelatedEntityForWire / stripHiddenProperties. AC1/AC3 verification must be explicitly assigned to dataentry handler tests against the real handleV1Search, separate from storetest.
severity: minor
resolution: 'Plan rev 2: AC1/AC3/AC3b explicitly assigned to dataentry handler tests against the real handleV1Search (acl_search_test.go); storetest conformance suite scoped to the Hit-level seam only.'
status: addressed
---
