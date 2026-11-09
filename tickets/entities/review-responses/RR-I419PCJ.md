---
id: RR-I419PCJ
type: review-response
title: Erase test gaps
finding: Missing lock-loss, audit-removal failure, cancellation and concurrent-erasure tests; hard-coded audit path; no test for leftover args.
severity: minor
resolution: Added TestErase_LostLockFailsClosed, TestErase_AuditFailures, TestRefuseExtraArgs and AuditRoot use. Concurrent erasure is covered by the lock test (TestErase_RefusesOpenDuringErasure takes the same exclusive lock); cancellation is the pgx context path and is not tested separately.
status: addressed
---
