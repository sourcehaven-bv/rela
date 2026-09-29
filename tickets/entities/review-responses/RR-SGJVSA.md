---
id: RR-SGJVSA
type: review-response
title: Relation purge audit carries the tail face in the summary only
finding: auditPurge puts the face into Summary; audit.Subject has no face field.
severity: minor
resolution: 'Deferred: the purge summary names the face (face=...) until audit subjects carry faces.'
reason: 'No audit subject in the project carries a face: entity and relation write audits omit it too. Adding a face field to audit.Subject is a cross-cutting audit schema change for all write ops, outside this bug. The purge summary names the face so the record is not ambiguous meanwhile.'
status: deferred
---
