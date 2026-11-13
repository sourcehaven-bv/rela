---
id: RR-C21IXY7
type: review-response
title: Realistic-ACL rows check skipped for known scans
finding: The realistic-ACL rows check is skipped when the fixture ACL rows already grow, so a scan only that ACL produces stays hidden there.
severity: minor
reason: Skipped only on the calls-only tests that already carry TODO(TKT-FED6FMM); the check applies again once TKT-FED6FMM removes those scans.
status: deferred
---
