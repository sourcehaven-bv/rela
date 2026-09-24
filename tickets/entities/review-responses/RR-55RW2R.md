---
id: RR-55RW2R
type: review-response
title: No tests for invalid data-entry.yaml or the CLI dry run on sqlite
finding: The destructive case CLAUDE.md calls out and ReconcileDerivedIndexes had no sqlite tests.
severity: minor
resolution: Added TestSQLiteInvalidDataEntryKeepsDerivedIndexes and TestSQLiteReconcileDryRun.
status: addressed
---
