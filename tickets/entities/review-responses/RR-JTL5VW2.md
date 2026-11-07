---
id: RR-JTL5VW2
type: review-response
title: Run gate does not stamp the world on the subject endpoint
finding: flowRunReadQuery uses gateHop without InWorld(world), unlike GateTraversal.
severity: minor
reason: Run subjects are faceless by construction, so the inherited selection reads the same row. readQuery has no world parameter; adding one belongs with the runtime wiring in TKT-9IT3LN8.
status: deferred
---
