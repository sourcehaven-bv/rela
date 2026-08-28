---
id: RR-PXW1PF
type: review-response
title: Per-clause indexes multiply write cost
finding: Creating one index for every query clause ignores compound query shape and can multiply indexes across static sources.
severity: minor
resolution: Plan canonicalizes one composite index per full entity-type and property-set query shape and deduplicates literal values.
status: addressed
---
