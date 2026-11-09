---
id: RR-DEBXVU1
type: review-response
title: 'Design: concurrent runs of one peer'
finding: The CLI and the scheduler could pull the same peer at once; state.KV has no compare-and-swap, so the last ledger save wins and rows lose their entries.
severity: significant
resolution: 'Plan: both paths create the run in schedulerstate for task peer:<name>; ErrRunActive refuses a second run.'
status: addressed
---
