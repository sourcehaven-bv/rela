---
id: RR-PZXQ9P
type: review-response
title: Empty EntityType returns nothing instead of every type
finding: e.type = '' matches no rows while graphquerynaive reads every type.
severity: minor
resolution: graphSource marks an empty EntityType unsafe, so every entry point including GraphCount falls back to graphquerynaive.
status: addressed
---
