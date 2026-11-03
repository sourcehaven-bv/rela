---
id: RR-RMIXVB
type: review-response
title: Face-qualified addresses depend on _self
finding: entityRef falls back to the bare id when _self is missing, which would make a faced write ambiguous now that writes carry no world.
severity: minor
resolution: Confirmed every entity response sets a face-qualified _self (entityserializer.go selfHref); documented the dependency in the api/piles.ts header.
status: addressed
---
