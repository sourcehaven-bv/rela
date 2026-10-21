---
id: RR-QTC1V8
type: review-response
title: SafeFS leftovers outside entities/ and relations/ are found by no sweep
finding: 'internal/analysis/analysis.go:593-597: rela gc --temp-files scans only EntitiesDir and RelationsDir. SafeFS also writes .rela/ state, comments, channels and config, and with random names a crash leftover there is no longer overwritten by the next write. The plan''s risk section says rela gc finds those; it does not. Fix: either extend FindOrphanedTempFiles to .rela/ and the comment/channel roots (they use the same predicate), or correct the plan and ticket text so the accepted risk is stated accurately.'
severity: minor
resolution: gc is not widened; the FindOrphanedTempFiles doc and the --temp-files help list exactly what it scans (entities/, relations/, attachments/), and the plan's risk text was corrected to say nothing sweeps .rela/, comments or channels.
status: addressed
---
