---
id: RR-6NFB3C
type: review-response
title: Every navigation re-runs all source queries
finding: Per-route resolution multiplies query load versus once per world.
severity: significant
resolution: 'Planned: client skips the request when the surface or view is disabled in config; server filters sources by placement, world and context type before any read; band short-circuit and stickiness keep it at one or two queries. Budget test pins source reads per request.'
status: addressed
---
