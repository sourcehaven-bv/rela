---
id: RR-3I27KMQ
type: review-response
title: Page tab and dashboard card value endpoint untested
finding: The dataentry test covered nav entries and the card list but not page tabs or the card data endpoint that share the same gate.
severity: minor
resolution: TestRoleRelationPermission_ShowsGatedUI now checks the page tab count and the card value endpoint (200 or 404).
status: addressed
---
