---
id: RR-GLMR91
type: review-response
title: Owner lookup reads every incoming edge
finding: The incoming relation query had no type filter.
severity: nit
resolution: The store filters by type when the schema has exactly one owning type.
status: addressed
---
