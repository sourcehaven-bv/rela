---
id: RR-49D3KF
type: review-response
title: IndexText indexes only Go string property values
finding: search.IndexText indexes only properties held as Go strings. The old trigger indexed every JSON string. The backfill reads JSON back, so a non-string value written fresh and the same value backfilled are indexed differently. Effect is a false negative, not a leak; pgstore behaves the same.
severity: minor
reason: Shared with pgstore and fails toward fewer matches, never a leak. Changing what IndexText indexes is a cross-backend change outside this bug.
status: deferred
---
