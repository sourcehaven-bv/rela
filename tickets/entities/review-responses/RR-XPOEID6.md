---
id: RR-XPOEID6
type: review-response
title: Schema-aware cleanup detections warn on valid configs
finding: Passing the schema to MigrateBytes enabled dataentry-cleanup detections that validate does not run, so valid configs warned on every load.
severity: minor
resolution: Pass nil, as before; the schema adapter move was reverted.
status: addressed
---
