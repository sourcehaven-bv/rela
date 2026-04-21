---
id: RR-LF2JF
type: review-response
title: Drive-letter check was position-1 only; Windows reserved names and ADS syntax bypassed
finding: 'resolve only caught X: at position 1. Colons elsewhere (ADS: foo:stream) and Windows reserved device names (CON, NUL, COM1-9, LPT1-9) were allowed. A key that works on POSIX would silently fail or open a device handle on Windows.'
severity: minor
resolution: 'Replaced narrow drive-letter check with strings.ContainsRune(key, '':'') which rejects colons anywhere (covers drive letters, ADS, and any other colon abuse). Added windowsReserved map with per-segment case-insensitive stem check (extension stripped). 6 new rejection test cases. Effort: ~15 LOC as estimated.'
status: addressed
---
