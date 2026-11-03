---
id: TKT-F8LRA78
type: ticket
title: 'pgstore: world-ranked single-id load spends ~0.8 ms planning per call'
kind: refactor
priority: medium
effort: s
status: backlog
---

## Description

Found while measuring TKT-QM833U. The world-ranked single-id load behind
`GetAddress` (`visibility.Resolver.loadInWorld`, pgstore SQL `SELECT ... FROM
(SELECT DISTINCT ON (id) ... WHERE ((type = $1 AND (face = $2 OR ...)) OR ...)
AND id = ANY($10) ORDER BY id, (CASE ...)) ...`) spends almost all its time in
planning. On the atlas-dev copy: about 0.8 ms bind per call against 0.02 ms
execute; `EXPLAIN (ANALYZE)` shows 2.9 ms planning and 0.09 ms execution. 776
such calls cost 606 ms of the 780 ms Postgres time of one `_analyze` request on
develop.

Likely causes: the OR/CASE predicate per faced type, and 39 indexes on
`entities` (derived list indexes) that the planner considers for each call.

Every `GetAddress` pays this, so it also affects single reads and Lua
`rela.get_entity`.

## Approach to evaluate

- Select by `id = ANY($1)` only and rank faces in Go (a family has few rows).
- Or keep the SQL and check whether a generic plan helps
(`plan_cache_mode`), measured on the atlas-dev copy.

Acceptance: bind + execute time per call drops by an order of magnitude on the
atlas-dev copy, with identical results (storetest conformance).
