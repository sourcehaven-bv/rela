---
id: RR-NMZ6HB
type: review-response
title: HighestIDIsExactPrefix cannot fail on an over-broad SQL range
finding: Every backend filters the SQL rows again in Go so the storetest subtest also passed on the old LIKE. It does not pin the SQL bounds or the sqlite ASCII case fold.
severity: minor
resolution: Added TestBuildHighestIDSQLBounds on pg and sqlite (FEAT- to FEAT.) and TestHighestIDFoldsASCIICase on sqlite (feat-9 counts for FEAT). The storetest subtest stays as the cross-backend semantic pin.
status: addressed
---
