---
id: RR-L2L9AR7
type: review-response
title: Range start was not aligned to a bucket
finding: now minus range falls mid-bucket, so the first sum or count point showed a false dip.
severity: significant
resolution: rangeStart floors to the UTC bucket start (hour, day, ISO week, month) like the server; tests per bucket and a Monday test.
status: addressed
---
