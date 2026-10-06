---
id: RR-CZ06NK
type: review-response
title: Empty-dest branch in newAccessLogger is dead code
finding: checkAccessLogDest accepts empty and openAccessLog returns early, so the constructor's empty check only ran in tests; the 'one allowlist' comment was inaccurate.
severity: nit
resolution: newAccessLogger is a switch over the two sinks with an error default; comments say which function owns which check.
status: addressed
---
