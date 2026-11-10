---
id: RR-SHLMJUL
type: review-response
title: A timeline in State is dropped by updates that rebuild State
finding: Every flowrun.Update call builds State from scratch; each would erase the timeline unless it copies the log.
severity: critical
resolution: Every engine update goes through one helper that carries the run's log and claim and appends an entry; a test fails on a direct Runs.Update call elsewhere in the package. The store contract is unchanged (full state per compare-and-set). Trimming keeps the start entry.
status: addressed
---
