---
id: BUGA-CVGPP9
type: bug-analysis-checklist
title: 'Analysis: pgstore filters faces before the world picks the prime'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Postgres test cluster, `go test -tags postgres -run
'TestConformance/Worlds/PropsFilter' ./internal/store/pgstore/`: pg returns PF-1
(default face) and counts 2; memstore and fsstore return only PF-2. Conditions:
a non-default world (after `effectiveWorld`) and at least one `Props` or
`Narrowing` predicate.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fix: `buildPredicateParts` splits its conjuncts into those that trim the
candidate faces before the rank (relation predicates, which are id-keyed, and
`Any`, whose branch face sets the docs define as pre-rank) and those that test
the prime (`Props`, `Narrowing`). Under a non-default world, rows, headers,
counts and `MatchingIDs` select the primes in an inner `DISTINCT ON` query and
apply the prime tests outside it. The default world keeps its single SELECT, so
its indexes and EXPLAIN plans are unchanged.

Regression test: the new storetest Worlds case runs on every backend.

Related areas: `ListEntities` takes no property predicates; pg visible search
takes no world; the TKT-XKCNCL list pushdown goes through these builders and is
fixed with them.
