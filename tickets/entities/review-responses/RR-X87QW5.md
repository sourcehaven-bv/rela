---
id: RR-X87QW5
type: review-response
title: Overlapping loadCandidates runs can overwrite newer results
finding: No generation guard; a slower earlier run could finish last and write stale candidates and hints.
severity: minor
resolution: Added a generation counter; a stale run neither writes results nor clears loading.
status: addressed
---
