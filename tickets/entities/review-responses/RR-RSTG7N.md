---
id: RR-RSTG7N
type: review-response
title: Poison channels block the due list and one damaged meta.yaml stops all timers of a type
finding: 'The list asks for MaxTimerWritesPerRound+1 rows ordered by due time. Failed writes leave the hint in place, so 65 persistently failing channels starve every later timer of that type forever, retried every round with a log line each. On filechannels, visitFaceAt returns on the first damaged meta.yaml (file.go:744-749), so ListSessionsDue fails for the whole type. Plan change: keep an in-memory per-key backoff for the leader term (skip a key that failed in the last N minutes, cleared by Reset), and have ListSessionsDue skip and warn on a damaged meta.yaml instead of failing the scan. Add tests for both.'
severity: minor
resolution: 'Plan updated: failing rows are logged, counted and put in a 5m in-memory backoff (cleared by Reset) without blocking the type; list limit grows by backoff keys; damaged meta.yaml skipped with a warning; tests added.'
status: addressed
---
