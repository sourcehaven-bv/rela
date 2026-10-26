---
id: RR-1F6OQP
type: review-response
title: AC 10 second clause and AC 19 'minus the cause read' cannot be constructed
finding: 'AC 10: a type that ''lost run_as entirely'' with job-move outputs still present does not load (AC 2), and with the outputs removed the mark is close-only, so it never reaches run_as_changed. AC 19: outputEngine.load reads the cause and the notice in one Messages page (internal/realtime/outputs.go:419-440), so a timer mark saves no read. Plan change: drop or reword both clauses; AC 19 should assert the same single page read.'
severity: nit
resolution: AC 11 now only covers a changed run_as (the lost-run_as clause is gone, as it cannot load); AC 20 asserts the same single Messages page read.
status: addressed
---
