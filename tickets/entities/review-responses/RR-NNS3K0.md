---
id: RR-NNS3K0
type: review-response
title: gc --scan aborted the whole command on routine sweep contention
finding: GC.Scan errors with ErrLockHeld on contention (by design), but the CLI propagated it and exited before Tick — so `rela migrate gc --scan` colliding with the server's hourly sweep failed with a bare error and never printed the dry-run report the operator wanted. A routine, expected collision on any sweep-enabled deployment.
severity: significant
resolution: 'The CLI now special-cases ErrLockHeld from Scan (commit d514cc8e): prints "scan skipped: another migration or GC run is active — re-run later" and continues to the tick preview. Real errors still abort. Scan itself keeps its error contract (it writes the ledger, so silent skipping inside the engine would be wrong for other callers).'
status: addressed
---
