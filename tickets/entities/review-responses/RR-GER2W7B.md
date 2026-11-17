---
id: RR-GER2W7B
type: review-response
title: ctx fallback hides a missed edge
finding: Read time silently falls back to a per-call clock when an edge is missed.
severity: significant
resolution: Filter variable binding takes an explicit time from readtime; edges stamp in the router and the binder; end-to-end tests pin the clock on search.
status: addressed
---
