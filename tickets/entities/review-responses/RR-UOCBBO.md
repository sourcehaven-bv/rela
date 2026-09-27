---
id: RR-UOCBBO
type: review-response
title: 'Review nits: comment spacing, datamigration comment, migrate rationale, pg SweepNow lock skip'
finding: Blank comment line in bulkmigrate; datamigration/run.go said 'on pg' only; addEditorColumns cited an impossible crash; pgstore SweepNow returned nil when the sweep lock was held elsewhere.
severity: nit
resolution: All four fixed; SweepNow now errors when the tick skipped for the lock.
status: addressed
---
