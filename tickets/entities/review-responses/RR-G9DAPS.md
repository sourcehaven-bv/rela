---
id: RR-G9DAPS
type: review-response
title: D5 load refusal can push the whole config into degraded mode
finding: '`FacesRestricted` is evaluated at load, so adding a face grant in acl.yaml makes all of realtime.yaml invalid. That either refuses the Configure save or degrades every trigger, condition and entity-property role, not just the affected trigger. Say so in the docs, and consider refusing only the affected rules.'
severity: minor
resolution: Only the affected trigger or auto rule is dropped with a load warning; the rest of realtime.yaml stays active.
status: addressed
---
