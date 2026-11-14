---
id: RR-S21OCEZ
type: review-response
title: Gate error does not name the entity type
finding: Rollups.gate returns the ReadQueryFor error without the type so an intermediate hop failure is indistinguishable.
severity: nit
resolution: Wrapped as read query for the type.
status: addressed
---
