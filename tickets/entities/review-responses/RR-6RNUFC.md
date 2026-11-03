---
id: RR-6RNUFC
type: review-response
title: Checks run one at a time
finding: collectProse checks entities serially, so the pool of 2 never helps.
severity: minor
reason: Startup dominates the cost; the batch-mode follow-up in TKT-EKY2TV replaces per-entity runs, so parallelising them now is short-lived work.
status: deferred
---
