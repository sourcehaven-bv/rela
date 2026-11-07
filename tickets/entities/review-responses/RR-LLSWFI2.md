---
id: RR-LLSWFI2
type: review-response
title: Permanent open errors are retried
finding: A malformed sealed state failed the job, which retried a failure that cannot change.
severity: minor
resolution: errDeletedStateCorrupt drops the job and erases the key.
status: addressed
---
