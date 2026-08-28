---
id: RR-5SD2TZ
type: review-response
title: pid reuse can wedge the fs lock with no documented remedy
finding: A crashed holder whose pid is recycled by an unrelated process reads as alive; the stale-break correctly refuses (never break a live pid) but the lock then stays wedged forever, and neither the code nor the guide named the operator escape hatch. A pid+starttime identity would close it fully but is platform-specific.
severity: significant
resolution: 'Accepted as a documented fail-safe limitation rather than platform-specific starttime probing (commit d514cc8e): the wedge direction only ever BLOCKS destructive work, never permits a double-run. The limitation and the remedy (`rm .rela/migration.lock` after confirming no run is active) are now documented both at breakIfStale''s godoc and in the data-migration guide''s crash-recovery paragraph. pid+starttime process identity (procfs/kinfo_proc) is the recorded upgrade path — in this RR — if the wedge proves common in practice.'
status: addressed
---
