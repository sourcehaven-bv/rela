---
id: TKT-ZYH3
type: ticket
title: 'store: GraphQuery DSL + naive impl (fsstore/memstore) + SQL-native pgstore + query tracer'
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Add a generic graph-shape query DSL to `store.Store`: `GraphQuery` /
`GraphCount`. The DSL takes no consumer vocabulary (no ACL, no search, no
analyze) so multiple consumers can compose against one stable shape.

```go
type GraphQuery struct {
    EntityType  string
    HasInbound  *RelationPredicate
    HasOutbound *RelationPredicate
}

type RelationPredicate struct {
    Endpoints []string
    OfTypes   []string

    // InheritThrough (endpoint-side): transitively expand Endpoints
    // via these relation types up to Depth before matching.
    // Example use: walking groups via member-of edges.
    InheritThrough []string
    Depth          int

    // EntityInheritThrough (entity-side): transitively expand each
    // candidate entity via these relation types up to EntityDepth;
    // a match against any ancestor satisfies the predicate.
    // Example use: walking containment via belongs-to edges.
    EntityInheritThrough []string
    EntityDepth          int
}
```

Both transitive expansions are independent and compose; together they form the
shape the future consumers (read-side ACL filtering, analyze tools, structured
search) need without bringing any of those vocabularies into the store layer.

## Scope

**In:**

- New `internal/store/graphquery.go`: types + `GraphQueryer`
interface embedded into `store.Store`.
- New `internal/store/graphquerynaive/`: backend-agnostic
iterate-and-filter implementation, exported `DepthCap = 5`.
- New `internal/store/{fsstore,memstore,pgstore}/graphquery.go`:
per-backend wrappers, each delegating to `graphquerynaive`.
- `internal/store/store.go`: embed `GraphQueryer` into `Store`
interface.
- `internal/store/storetest/graphquery.go`: conformance suite
(`RunGraphQueryTests`) covering direct match, both transitive expansions,
both-compose, OfTypes filter, cycle/self-loop termination, Depth=0 no-op, and
`GraphCount` matched-vs-total. Wired into `RunAll` so every backend runs it for
free.
- `.go-arch-lint.yml`: declare `graphquerynaive` component +
`mayDependOn: [entity, store]`; add to fsstore/memstore/pgstore `mayDependOn`.

**Out (deferred):**

- Push-down implementations. The pgstore wrapper delegates to naive
today; a SQL-native recursive-CTE impl is a natural follow-up ticket (the
package docstring flags this).
- Index-backed fsstore impl (depends on a future indexes ticket).
- Any consumer of the DSL: future ACL read filtering, analyze
tools, etc. — out of scope for this PR.

## Acceptance criteria

- AC1: `store.Store` embeds `GraphQueryer`; all three backends
(memstore, fsstore, pgstore) compile against the new interface via build tags.
- AC2: `RunGraphQueryTests` passes for all three backends — same
result set for same input. Includes self-loop, cycle, Depth-zero,
both-expansions-composed cases.
- AC3: `GraphCount` returns `(matched, total)` where `total`
ignores predicates.
- AC4: arch-lint clean; lint clean (0 issues); `just ci` clean.
- AC5: `RELA_TEST_DATABASE_URL=... just test-postgres` passes
with the new conformance suite running against real postgres.

## Why now

The store DSL is the foundation under several near-term consumers (ACL v1 read
filtering, future structured search). Landing it first as a small, consumer-free
PR pins the contract before any consumer arrives, keeps the interface generic,
and lets the push-down work happen later without coordinating consumer changes.

## References

- Companion follow-up: pgstore SQL-native GraphQuery (recursive
CTEs + composite indexes) — to be filed once this lands.
- The Python prototype at `.ignored/acl-prototype/store.py` was
the design validator for this DSL shape.
