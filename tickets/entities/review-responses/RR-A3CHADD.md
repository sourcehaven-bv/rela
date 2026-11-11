---
id: RR-A3CHADD
type: review-response
title: Scope list misses CalDAV and channel writes
finding: CalDAV client edits write through the plain manager with no field check (caldav_write.go:419, 1033); channel promote and outputs are field-gated (realtime_promote.go:50, realtime_outputs.go:63) and will be narrowed. Neither is listed.
severity: significant
resolution: 'Addressed in plan: CalDAV added to the not-narrowed list; channel promote and outputs listed as narrowed and documented.'
status: addressed
---
