---
id: RR-HAHY15G
type: review-response
title: Prune vs create lock on pg unspecified and per-flow lock too costly
finding: Per-flow exclusive locks serialize all starts of a flow and prune across flows can miss flows; the FK alone aborts prune under READ COMMITTED.
severity: significant
resolution: 'One per-schema advisory key: Create takes it shared and PruneDefinitions exclusive; FK stays as a backstop; harness races prune and create.'
status: addressed
---
