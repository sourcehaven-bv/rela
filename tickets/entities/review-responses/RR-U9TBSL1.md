---
id: RR-U9TBSL1
type: review-response
title: Check that settle cannot run twice
finding: The create settle and the flow worker can both pick up a new run.
severity: minor
reason: 'No change needed: settleStep claims each step with a compare-and-set on the run''s rev before writing outputs (step.go), the same claim a manual start relies on.'
status: wont-fix
---
