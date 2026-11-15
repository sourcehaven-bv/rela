---
id: RR-QPW5ADO
type: review-response
title: Faced subject and target types unhandled
finding: Store reads and writes take entity.Ref with a face (DEC-NPZICR). The plan does not say how a faced subject or faced candidate type is addressed. Reject faced types for subject and relate targets in v1, as flows do.
severity: significant
resolution: Config load rejects a faced subject type and faced relate target types. Reads use the caller's world (store.InWorld / GetAddress).
status: addressed
---
