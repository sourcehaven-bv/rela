---
id: RR-R32CJOI
type: review-response
title: Log file left behind on Ctrl-C
finding: TEST_LOG is not cleaned up on INT or TERM.
severity: nit
reason: A temp file in TMPDIR after an interrupted run is harmless; the OS cleans TMPDIR.
status: wont-fix
---
