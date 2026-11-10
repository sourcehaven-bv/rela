---
id: RR-BNM4ZMM
type: review-response
title: Check and write are not atomic
finding: A run that starts between the ceiling check and the write does not stop that write.
severity: minor
reason: Same check-then-write shape as every field rule in the manager; closing it needs the run lookup inside store.Tx, which the Tx rules forbid for slow I/O. Documented in GUIDE-flows.
status: wont-fix
---
