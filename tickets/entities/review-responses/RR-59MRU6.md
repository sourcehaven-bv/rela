---
id: RR-59MRU6
type: review-response
title: MCP tools disagree on a bare faced id
finding: update_entity by bare faced id misses (Resolve; ruling 7.1) while delete_entity and rename_entity accept it (Family).
severity: nit
reason: 'Intended Stage 1 behavior: entity-level operations take a bare id and face reads need an address until TKT-7IZHP0 gives the default world a face. The tool descriptions change with TKT-7IZHP0.'
status: deferred
---
