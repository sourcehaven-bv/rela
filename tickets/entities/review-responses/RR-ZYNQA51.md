---
id: RR-ZYNQA51
type: review-response
title: NaN disables the fix and the fallback is silent
finding: DeepEqual(NaN / NaN) is false so a file with .nan is never re-wrapped / and nothing is logged on fallback.
severity: minor
resolution: Fallback now logs at debug level. NaN in config files is rare and the fallback keeps the old behaviour. Test case pins the fallback.
status: addressed
---
