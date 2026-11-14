---
id: RR-55LGO9J
type: review-response
title: Title read ignores the world
finding: The picker searches with worldParam but getEntity read the default world.
severity: minor
resolution: 'getEntity now passes { world: worldParam }.'
status: addressed
---
