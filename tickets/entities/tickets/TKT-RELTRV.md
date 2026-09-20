---
id: TKT-RELTRV
type: ticket
title: 'Relation traversal in conditions: related() lowering to a nested store predicate with SQL pushdown'
kind: enhancement
priority: medium
effort: xl
tags:
- needs-design
- security
status: done
---

## Description

Add relation traversal to the condition language so a condition can filter on a
related entity's properties, evaluated as a pushed-down SQL `EXISTS` rather than
a per-row Go lookup.

Full design, measurements and rejected alternatives: **RES-RELTRV**.

### Surface

```lua
related(entity, 'caused-by', { type = 'ticket', status = 'done' })
related(entity, { 'rel-x', 'rel-y' }, { type = 'C', prop = 'foo' })
```

Valid Lua; recognised at COMPILE time and lowered to a traversal IR node, NOT a
runtime host function — a non-`SQLPortable` host func taints the whole program
(`internal/predicate/program.go:96`) and would disable pushdown for every other
clause in the same condition.

### Scope (v1)

- ANY semantics (`EXISTS`); no "all"
- equality/inequality only; ordered comparison deferred with the nested-operator
  table form (`{ due = { lt = today() } }`)
- chaining allowed, depth capped at compile time against `DepthCap`
- `type =` ascription MANDATORY when `RelationDef.To` has >1 element, optional
  otherwise
- **refuse to compile** when a neighbour property is not pushdown-eligible,
  rather than silently falling back to Go per-row evaluation

That last point is load-bearing. A silent Go fallback is the N+1 the
collection-reads rule exists to prevent, and it would be invisible to
`storetest.Counting` budget tests because it happens above the store's query
path. Failing at config load, naming the property, matches how `condition:`
compile failures are already treated as load errors.

### Work breakdown

| Area | Change | Risk |
|---|---|---|
| `internal/predicate` | Traversal IR node + static recognition of the form | Low — table-arg walking exists (`walk.go:452`) |
| `internal/store` | Nested predicate field on `RelationPredicate` | Low |
| `pgstore` | Join + conjuncts in EXISTS; `propCond` alias param (hardcodes `e.`, `graphquery.go:331`) | Medium — shared with `visiblesearch.go` |
| `graphquerynaive`, `sqlitestore` | Backend parity, must pass `storetest` | Medium |
| `queryplan` | Target-type eligibility gate + SECOND derived-index spec | High — new logic, no existing helper |
| `visibility`/`acl` | Per-hop ReadQuery composition, filterability rule | High — security-critical |

### Index behaviour (settled, measured on PG 18.6)

The derived index is PARTIAL on `type = <T> AND jsonb_typeof(...) = 'string'`.
Postgres matches a partial index only when the query implies its WHERE clause,
so the lowering MUST emit the `jsonb_typeof` scalar guard or the index is not a
candidate at all:

| query shape | plan | time |
|---|---|---|
| no guard | bitmap scan, 4990 rows filtered | 1.725 ms |
| with guard | `Index Scan using rela_derived_query__concept_status` | 0.541 ms |

Reuse `propCond`'s scalar spelling (`PropPredicate.Scalar`) rather than
hand-rolling the comparison — which makes the `propCond` alias refactor
mandatory, not cosmetic. `StaticIndexSpecs` must also emit a second
`DerivedObjectSpec` for the traversed-TO type; without it the join degrades to a
full bitmap scan of the neighbour population. No new index SHAPE is needed.

### ACL (security-critical)

A traversal filter over a neighbour's property values is an INFERENCE CHANNEL,
distinct from the `Any`/`Narrowing` privilege split. The row gate is composed
per-type — `ReadQueryFor(ctx, q.Type)` for exactly one type
(`internal/visibility/pushdown.go:73`) — and the resulting query's relation
predicates carry no gate of their own.

The leak is not a disclosure: a traversal returns A-rows, so response redaction
never fires. It is the CORRELATION between filter and result set, and it is
binary-searchable.

Three cumulative mitigations, all required:

1. **Per-hop row gate** — compose `ReadQueryFor(targetType)` for each hop and AND
   it into that hop's EXISTS body. `DenyAll` makes the traversal match nothing;
   `AllowAll` contributes no conjunct. Cost is per query-build, not per row.
2. **Filterability rule** — a property whose visibility is conditional
   (`visible:`) must not be filterable. Independent of (1): the row gate does not
   help when the principal can see the row but not the field.
3. **No traversal on ACL-evaluated surfaces in v1** — affordance `when:` /
   `visible:` predicates run INSIDE the gating decision, so gating them needs the
   gate they produce.

Separately: a caller-supplied traversal lowers into `GraphQuery.Narrowing`,
NEVER `Any` — deliberately different types so the mistake cannot compile
(`internal/store/graphquery.go:77`).

### Open

- No metamodel helper exists for "properties common to a set of types with
  identical declared type"; `GetRelationTo` exists
  (`internal/metamodel/schema_output.go:106`). This plus the union rules is the
  real new work, not the SQL.
- Whether the frontend view/kanban `where:` surfaces need traversal. If so it
  permanently rules out any Go-evaluated fallback.
