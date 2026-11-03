---
id: RR-DDWDAH
type: review-response
title: AC1's "empty string is refused" needs a pointer field
finding: 'With `rawSessionState.Status string`, `status: ""` cannot be told apart from a missing `status:`, so the empty-string row of `TestSessionStatus_Values` cannot pass. Change: use `Status *string`, as `session.run_as` already does (`realtimecfg/grants.go:45`, `raw *string`), or drop the row.'
severity: minor
resolution: rawSessionState.Status is *string, as run_as is, so an explicit empty status is refused (AC 1).
status: addressed
---
