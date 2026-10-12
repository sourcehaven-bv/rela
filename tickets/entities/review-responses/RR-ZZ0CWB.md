---
id: RR-ZZ0CWB
type: review-response
title: Two-instance tests share one process semaphore
finding: sems is keyed by absolute path process-wide, so two Store instances never contend on the Locker; a broken filelock still passes. Use a subprocess or a test constructor bypassing sems.
severity: minor
resolution: Disk test stores get their own semaphore so only the file lock separates the instances.
status: addressed
---
