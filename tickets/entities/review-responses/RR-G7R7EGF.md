---
id: RR-G7R7EGF
type: review-response
title: '[security] A branch step is exempt from its sibling branches'' outside_edits'
finding: '[security] WithinRun exempted the whole run, so in a forked run a branch step or output skipped the outside_edits rules of the sibling branches'' states.'
severity: minor
resolution: WithinRun now takes the state the write comes from; Withheld skips only that state of the run. Outputs.Set carries the state. New TestCeiling_Withheld case for a sibling branch.
status: addressed
---
