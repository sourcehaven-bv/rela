---
id: RR-R5AR5GN
type: review-response
title: Per-edge POST builds a structuralError and drops its path
finding: handleV1CreateRelation uses only Code and Detail of the structuralError from endpointTypeRefusal.
severity: nit
reason: Sharing the one mapping keeps the code choice in one place; the unused Path is harmless.
status: wont-fix
---
