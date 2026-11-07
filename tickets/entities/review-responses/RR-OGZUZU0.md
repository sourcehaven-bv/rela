---
id: RR-OGZUZU0
type: review-response
title: 'Code: the unfilled-row path of a stored type is untested'
finding: The Fingerprint fallback when the aggregate reports pending rows and relationViews were barely covered; elementnaive never returns an unfilled element so the backfill path of the database backends had no test.
severity: significant
resolution: TestElements_StoredTypeWithUnfilledRows wraps elementnaive to report unfilled and unparsable stored hashes for entities and relations and checks Fingerprint and List equal the filled answer.
status: addressed
---
