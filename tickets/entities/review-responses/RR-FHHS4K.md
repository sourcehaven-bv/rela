---
id: RR-FHHS4K
type: review-response
title: List payload heavier than needed
finding: The list handler loads page edges and neighbour visibility and serializes all properties and relations (api_v1.go:785-806).
severity: minor
resolution: Accepted; included in the cost measurement. A lighter projection is out of scope unless the measurement says otherwise.
status: addressed
---
