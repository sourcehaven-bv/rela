---
id: TKT-F4TIS6
type: ticket
title: Extend store.GraphQuery with property predicates and relation negation
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Goal

`store.GraphQuery` is **relation-shaped only** today
(`internal/store/graphquery.go:20`): `EntityType` plus `HasInbound` /
`HasOutbound` relation predicates with transitive expansion. It was built for
the ACL read gate, and it has no way to express a **property** condition or a
**negated** relation condition.

Add both, in every backend, behind the existing conformance suite.

## Why it matters beyond next-actions

This is the load-bearing gap for [[FEAT-79DTF9]] Phase 1 — property predicates
appear in 5 of the 7 scenario shapes surveyed in [[RES-09YLLL]] — but it is
independently useful:

- The ACL read gate can push more of its work down
- Dashboard cards get a fast filtered count
- `ViewTraverse.Where` (`internal/dataentryconfig/config.go:665`) currently
filters Go-side

Relation negation in particular is the shape the whole `analyze_orphans` /
cardinality family is built on ("entities with no `implements` edge"), so it has
callers today.

## Why not the alternatives

- **`search.SearchVisible` filters do NOT push down.**
`internal/store/pgstore/visiblesearch.go:44-47` documents that `q.Filters` are
applied Go-side, and that the SQL `LIMIT` is dropped when filters are present
(to avoid re-opening a starvation gap). Explicitly not the fast path.
- **Go-side filtering over a relation-narrowed candidate set** degenerates to a
full type scan whenever there is no relation predicate — which is the common
case for a plain `type + property` query.

## Scope

1. **Property predicates** on `GraphQuery` — a `Props []PropPredicate` field or
equivalent. Semantics must match `internal/filter`, which already defines
empty-vs-absent: *"missing or empty properties do NOT match any filter, except
when explicitly checking for empty values with `property=`"*
(`internal/filter/match.go:31-41`). `property=` → is-empty, `property!=` →
is-not-empty. **Do not invent a second notion of empty.**
2. **Relation negation** — express "has no matching inbound/outbound edge".
Note `HasOutbound: nil` currently means *unconstrained*, so negation needs its
own representation (e.g. a `Negate bool` on `RelationPredicate`), not an
overload of nil.
3. **pgstore**: extend `buildGraphQuerySQL` — property conditions belong in the
`WHERE`, indexable; negation as `NOT EXISTS`.
4. **graphquerynaive**: the fs/mem equivalent, same semantics.
5. **Conformance suite**: extend `storetest` coverage so both implementations
are held to one contract, including the empty-vs-absent edge cases and negation
interacting with transitive expansion.

## Constraints

- **Backend parity is the point.** The default build (`rela`, `rela-server` —
most users) must answer identically to postgres. A capability only pgstore can
serve breaks the default build or silently diverges. Resist "just add it in SQL,
it's only for the fast path".
- **`GraphCount` and `MatchingIDs` must honour the new predicates too** — they
share the query shape.
- Keep `GraphQuery` a value type; implementations must not mutate it
(`MatchingIDs` already documents this).

## Out of scope

- **Far-side predicates** (a condition on the *neighbour's* properties, not the
candidate's) — this is a join with a filter on the far end, and it is where
query languages go to grow forever. Deliberately excluded; see scenario S7.
- **Date arithmetic** (`days_since`) — separate scope, meets this work at this
seam.
- **Permission-filtered candidates** ("where the principal has write
permission") — a third axis, deferred, and the most delicate: it must be
consistent with the real ACL evaluation, never a reimplementation. Two
permission engines that disagree is a security bug.

## Acceptance

- Both backends pass the extended conformance suite
- A `type + property` query is index-backed on postgres (verify via the existing
`graphquery_explain_test.go` approach)
- `just arch-lint`, `just ci` green; postgres suite via `just test-postgres`
