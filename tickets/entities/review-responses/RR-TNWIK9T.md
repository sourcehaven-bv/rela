---
id: RR-TNWIK9T
type: review-response
title: 'Design: a branch reaching a final state must end the run in one write'
finding: finish patches the header and then deletes on the old rev; a sibling write in between makes the delete conflict, leaving a completed header over a live run.
severity: significant
resolution: 'Plan updated: a move to a final state from a branch, and a join that fires, are one compare-and-set that drops the branches and moves the run-level instance into the target with a pending mark; the existing finish path then ends the run.'
status: addressed
---
