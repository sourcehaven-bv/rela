---
id: RR-OVXRYQ
type: review-response
title: Persistent shape conflicts take the global lock per write
finding: A conflict is not cached, so every write to a conflicting type takes the advisory lock and reads all records.
severity: nit
reason: A persistent conflict is an operator misconfiguration that boot and reload already refuse; per-identity rate limits bound the cost meanwhile.
status: wont-fix
---
