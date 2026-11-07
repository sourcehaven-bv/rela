---
id: RR-Z7SEMB6
type: review-response
title: migrationLockDir fix is not exercised by any test
finding: Reverting migrationLockDir left the suite green; the fixture comment claimed a pending migration the world never sees.
severity: significant
resolution: Added TestInMemoryFS_SelectsProcessLocks (MemFS gives no lock dir; disk and SafeFS(disk) give CacheDir). Fixture comment now says the world reads an empty MemFS record and the disk record exists so the tree check covers applied.json.
status: addressed
---
