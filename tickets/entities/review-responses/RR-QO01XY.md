---
id: RR-QO01XY
type: review-response
title: serializeRelatedEntityForWire relation leak not covered by hit filter; conformance suite tests wrong layer for AC1/AC3
finding: The hit filter only decides which root entities survive. entityToV1 populates Relations (relation-type → target IDs) when includeRelations=true; handleV1Search currently passes false but the plan never pins that invariant, and the storetest conformance suite operates on search.Hit{ID,Type,Title} only — it structurally cannot catch a visible hit exposing a hidden entity's ID/title through its serialized body. AC1/AC3 are dataentry-serializer guarantees assigned to the wrong test layer.
severity: critical
resolution: 'Plan rev 2: includeRelations=false pinned by comment + test (no longer convention). New AC3b: visible hit relating to a hidden entity exposes no hidden ID/title/property through any serialized field (raw-body assertion in dataentry handler test). AC1/AC3 verification explicitly assigned to dataentry handler tests; conformance suite scoped to Hit-level seam only.'
status: addressed
---
