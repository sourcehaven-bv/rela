---
id: TKT-QM833U
type: ticket
title: GET /api/v1/_analyze runs thousands of statements per request on Postgres
kind: refactor
priority: high
effort: m
started: "2026-10-24"
completed: "2026-10-31"
status: done
---

## Description

`GET /api/v1/_analyze` is slow in production (atlas, rela with the Postgres
backend). Measured from the atlas access log over 12-19 Oct 2026:

- 305 requests over 1 s, 287 of them over 5 s, max 16.3 s.
- About 80 % of request time is spent in the database.
- It occurs every day (23 Oct: 17 requests over 1 s, max 4.6 s; 19 Oct: 28
over 1 s, max 12.4 s).
- No single statement crosses Postgres' 500 ms slow-query log, so the cost is
many statements per request or repeated full scans.

The SPA dashboard calls `_analyze` on every visit and waits for it before it
renders its cards (`frontend/src/views/DashboardView.vue:109-111`), which
explains why it shows up every day.

## Reproduction

Local Postgres, copy of `atlas-dev` (663 entities, 1,238 relations, last edit 20
Oct 2026) in database `rela-perf-analyze`, atlas config from
`~/Work/sourcehaven/atlas` (78 validation rules, `role_relations`
`toegewezen_aan`), scheduler disabled. Principal: jvloothuis (5 roles).
`rela-server-postgres -access-log=stderr`, five runs each:

| Build                         | statements | wall ms     | db ms       |
| ----------------------------- | ---------- | ----------- | ----------- |
| develop @ 47206052b            | 12,046     | 1,470-1,960 | 1,270-1,720 |
| + fix A (prime pushdown)      | 3,356      | 1,240-1,280 | 1,060-1,090 |
| + fix A + B (batch titles)    | 2,207      | 775-940     | 610-750     |

A and A+B returned a byte-identical response to develop. Another principal
(wwarendorff, 1 role) costs 6,198 statements on develop and 3,328 with fix A.

## Where the 12,046 statements come from

Attributed with seqtrace (`tools/seqtrace`, postgres build) and the
per-statement Postgres log (`log_min_duration_statement = 0` on the scratch
database only):

1. **8,808 (73 %): validation candidates are redacted one row at a time
without a local-role prime.** `GenericValidator.loadCandidates` lists each
rule's type through `ScriptReader.ListEntities`, which takes the ACL pushdown
path (`visibility.listPushdown` → `redactingSeq`). That path calls `RedactRow`
per row and, unlike `PolicyReader.Filter` and `FilterHeaders`, never calls
`PrimeTraversals`. Every field verdict therefore runs
`acl.Request.computeForEntity` → `localRoles` → `aclstore.Graph.HasEdge`: 8,730
single-edge `GetRelation` lookups for 1,459 candidate rows (78 rules reload
their type, so a row is redacted once per rule).
2. **1,818 (15 %): Lua validation scripts.** `procedure-heeft-open-taak` and
`procedure-heeft-terugkerend` call `rela.get_relations` and then
`rela.get_entity` per edge, for each of 86 procedures. This is atlas config;
rela cannot batch it behind the per-entity script contract.
3. **1,161 (10 %): `analyzeValidations` re-loads every violation with
`GetAddress`** (387 calls), also past the 100-issue cap that `capIssues` applies
afterwards. 287 of those loads are discarded.
4. **180: `analyzeOrphans` re-loads up to 101 orphans one by one.**
5. About 80 for the rest (properties, duplicates, gaps, cardinality).

Server-side Postgres time is about 780 ms of the 1,400 ms db time; the rest is
round trips. Of the 780 ms, 606 ms is one statement shape: the world-ranked
single-id load in `visibility.Resolver.loadInWorld` (776 executions). It is
almost all **bind time** (planning, about 0.8 ms each; `EXPLAIN` shows 2.9 ms
planning against 0.09 ms execution). The query has an OR/CASE predicate per
faced type and `entities` has 39 indexes. Fixes A and B remove most of those
calls; the remaining 332 come from Lua `get_entity`.
