---
id: RR-B5HOUM
type: review-response
title: 'Design: comments panel addresses the bare id'
finding: 'Probe: the detail page requests GET /_comments/policy/POL-1 (404) because the _views entry carries no _world, so commentEntityId falls back to the route id. The task lists comments on a face as expected to work; it does not.'
severity: minor
resolution: Covered by BUG-FYEEVX (the comments panel is its first listed site). The spec is written live and marked fixme with that id; the evidence goes in the report.
status: addressed
---

Probe: the detail page requests GET /_comments/policy/POL-1 (404) because the
_views entry carries no _world, so commentEntityId falls back to the route id.
The task lists comments on a face as expected to work; it does not.
