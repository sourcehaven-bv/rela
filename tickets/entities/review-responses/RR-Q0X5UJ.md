---
id: RR-Q0X5UJ
type: review-response
title: Each delete wake runs the full hourly retention including orphan and type scans
finding: realtime_worker.go:339 calls RunRetention, which also runs detachOrphans (Keys over all channels, Faces for every id, ListMarked) and clearTypeRecords (a full tree walk per undeclared type on fs) at retention.go:189. A bulk delete turns the hourly O(all channels) cost into back-to-back scans; a wake should run only the purge passes.
severity: significant
resolution: A delete wake now runs only RunPurge; orphan and type scans stay on the hourly retention run.
status: addressed
---
