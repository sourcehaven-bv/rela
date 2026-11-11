---
id: RR-4FMHHR3
type: review-response
title: Post rate checks use a stale clock
finding: Post read the clock before the file checks, which can take seconds with a scan.
severity: minor
resolution: Post restamps c.now after checkFiles.
status: addressed
---
