---
id: RR-A05G10F
type: review-response
title: '[security] Docs miss broad read grants and past rows'
finding: '[security] The capture docs did not warn that a role with read: ["*"] also reads the metric entity, or that past rows stay readable after access to the source data narrows.'
severity: minor
resolution: Added both points to the records and scheduled-tasks guides.
status: addressed
---
