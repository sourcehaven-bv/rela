---
id: RR-E82DGU
type: review-response
title: rela gc --temp-files and analysis are not aligned with the new rule
finding: analysis.FindOrphanedTempFiles and findTempFilesInDir (internal/analysis/analysis.go, used by internal/cli/gc.go) only match .new, which no current code writes, and have no grace period. After this change, the manual gc tool still cannot find .tmp leftovers, while the automatic sweep can. If gc is later extended to .tmp without the grace period, it brings back bug 1 (it deletes a temp file another process is about to rename). Use the shared storage predicate and stale-after rule from the first finding in analysis too. Either make gc sweep stale .tmp files, or state in the plan why gc stays .new-only.
severity: minor
resolution: 'Plan updated: analysis.FindOrphanedTempFiles uses the shared predicate and stale rule (AC6).'
status: addressed
---
