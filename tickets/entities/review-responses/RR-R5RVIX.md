---
id: RR-R5RVIX
type: review-response
title: The single-process invariant is per-project, not per-machine — and flock can silently fail on network filesystems
finding: 'The comments justifying the three inherited !postgres no-ops say ''sqlitestore refuses a second process'' without qualification. The lock is per-project-path: two rela-sqlite processes on DIFFERENT projects both run happily, which is correct and desirable, but means the invariant is ''one writer per project'', not ''one process''. That precision matters because derivedschema_nosweep.go explicitly warns a future reader that removing the lock creates a silent correctness hole, and the warning is only actionable if its scope is stated exactly. Separately: flock is unreliable on NFS and some FUSE mounts, where it may silently succeed for everyone -- and then the uniqueness scan has no backstop at all.'
severity: minor
resolution: 'The scope qualifier is worth adding and the derivedschema comment now says ''one writer per project database'' rather than ''one process''. On the network-filesystem half: this is already closed, and by the mechanism the reviewer suggested — Open verifies PRAGMA journal_mode actually returned ''wal'' and REFUSES to start otherwise, which is exactly the case where flock cannot be trusted. That guard predates this review (it is the arm-F mitigation from TKT-TWIO11) and is covered by TestWALIsEnabled.'
status: addressed
---
