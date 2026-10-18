---
id: RR-RNF0TT
type: review-response
title: Purge audit erases filed history and writes into the trail of a possibly reused id
finding: A2 Purge deletes the filed audit trail, which contradicts the T2 decision that audit is never deleted with a channel. It then appends AuditDelete records to the live trail of the id, which EntityCreated tombstoning implies may now belong to a new entity that should "start clean". Decide whether audit survives purge and write the purge record into the batch's own (kept) trail or a system trail.
severity: minor
resolution: Purge keeps the filed audit trail of the batch and appends the AuditPurge record to that trail, never to the live trail of the id.
status: addressed
---
