---
id: RR-5NICGWU
type: review-response
title: Batch response with all rows skipped becomes a 204
finding: writeActionResponse writes 204 for an empty response, which the SPA cannot tell from a script action.
severity: minor
resolution: 'Plan: proposals is always set for a batch run.'
status: addressed
---
