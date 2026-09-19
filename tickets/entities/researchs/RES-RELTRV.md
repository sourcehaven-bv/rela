---
id: RES-RELTRV
type: research
title: 'Relation traversal in the condition language: table-form syntax, SQL pushdown, and the neighbour-filter inference channel'
summary: 'Add related(entity, path, {constraints}) lowering statically to a nested store predicate; the union-typing of multi-target relations and the ACL gate on the traversed type are the two hard parts, not the SQL.'
status: done
---

## Problem

The condition language (`internal/predicate`) cannot reach the far side of a
relation. Given `A -rel-x-> B`, no condition can express a filter on `B`'s
properties. The goal is to add that, **with SQL pushdown as the primary
requirement** — traversal must compose into `store.GraphQuery` and be evaluated
by the database, not row-by-row in Go.

Four questions:

1. **Syntax** — what does a traversal look like, given the language must stay
   valid, logical Lua?
2. **Typing** — a relation's target is a SET of entity types. What does a
   property reference mean when the set is heterogeneous?
3. **Pushdown** — what does the traversal lower to, and does it use an index?
4. **ACL** — can a traversal filter leak data the principal cannot read?

Questions 2 and 4 are the hard ones. The SQL (question 3) is nearly free.

## Context

### What exists today

Relation-awareness in conditions is limited to two host functions in
`internal/affordances/hostfuncs.go`:

- `has_relation(entity, 'rel-x')` — does any outgoing edge of this type exist?
- `count_relations(entity, 'rel-x')` — how many?

Both are backed by `RelationLookup.OutgoingCounts`
(`internal/appbuild/relationlookup.go:27`), which tallies edges by type in a
single scan and **never loads a neighbour**. Reaching `B.prop` is impossible by
construction, not by oversight.

### Three structural constraints

**The engine is dependency-free on purpose.** `internal/predicate/arch_test.go`
forbids importing `store`, `entity`, `metamodel`, `tracer`, `acl`. The engine is
a pure function from (Program, Bindings) to a Value, with no I/O. Traversal is
inherently I/O, so it cannot execute inside `predicate`.

**Host functions cannot return records.** `Env.DeclareFunc`
(`internal/predicate/env.go:164`) rejects record and list return types (RR-93UN):
the runtime type check does not recurse into a returned record's fields, so
`related(...).prop` could observe a field whose runtime type contradicts its
declared type. The obvious design — a function returning B — is blocked by a
deliberate soundness guard.

**A host function would defeat the goal.** A `callNode` whose signature is not
`SQLPortable` taints the WHOLE program (`internal/predicate/program.go:96`),
disabling pushdown for every other clause in the same condition. Implementing
traversal as a runtime host function would make conditions that currently push
down stop pushing down. This rules out the entire "add a function" family of
designs.

### The store is closer than expected

`buildPredicateSQL` (`internal/store/pgstore/graphquery.go:557`) already emits
`EXISTS (SELECT 1 FROM relations r WHERE ...)` per relation predicate, with
recursive CTEs for the two inheritance expansions. The traversal filter is one
additional conjunct inside that EXISTS body:

```sql
EXISTS (SELECT 1 FROM relations r
        JOIN entities b ON b.id = r.to_id          -- new
        WHERE r.rel_type = ANY($1)
          AND r.from_id IN (...)                    -- existing
          AND (b.properties ->> 'prop') = $2)       -- new
```

`store.PropPredicate` already carries the full operator set with settled
semantics, including the `PropNotEqualOrEmpty` distinction that exists
specifically so a `predicate` `~=` lowers soundly
(`internal/store/graphquery.go:232`). Traversal inherits that correctness work.

`EXISTS` also answers the to-many question for free: it means ANY. ("All
neighbours satisfy P" would need `NOT EXISTS (... AND NOT P)` — out of scope for
v1 rather than guessed at.)

### Chaining is already tolerated by the graph layer

Multi-hop is not the cost problem intuition suggests. `DepthCap = 5`
(`internal/store/graphquerynaive/naive.go:46`) already bounds transitive walks,
with a visited-set as the primary termination mechanism, and its doc explicitly
contemplates deep chains and fan-out. A fixed-length chain of N literal hops is
**strictly easier** than the existing `InheritThrough`/`EntityInheritThrough`
expansions, which are transitive closures over unknown depth; a static chain has
a compile-time-known N. Nested `EXISTS` is what Postgres flattens into
semi-joins routinely.

### Multi-target relations are common, and their unions conflict

`RelationDef.To` is `[]string` (`internal/metamodel/types.go:1257`). Measured
against the live `tickets/schema.yaml`: **47 relations, 15 of them (32%)
multi-target**, up to 5 types wide (`detected-by`).

Property overlap across those unions is poor, and the failure is specific:

| relation | targets | common props | union | type-conflicting |
|---|---|---|---|---|
| `caused-by` | concept, decision, ticket, bug | 2 | 19 | `status` |
| `depends-on` | ticket, bug, feature, doc-task | 3 | 14 | `status` |
| `detected-by` | 5 types | 2 | 12 | `status` |
| `triggered-by` | ticket, bug, feature, decision | 2 | 17 | `status` |

**14 of the 15 multi-target relations have `status` in their conflict set.**
`status` is declared on nearly every type but with a DIFFERENT enum type on each
— `ticket_status`, `bug_status`, `feature_status`, `concept_status`.

This is the decisive finding. `entity.caused_by.status == 'done'` is the most
natural expression an operator would write, and it is exactly the one that
cannot be soundly typed: `'done'` may be valid in `ticket_status` and meaningless
in `concept_status`. The engine coerces literals against the operand's declared
type at COMPILE time; with a union operand there is no single type to coerce
against.

### Index behaviour is settled: it works, but only with the scalar guard

Measured on PostgreSQL 18.6 against a synthetic schema mirroring the real one
(migration indexes reproduced verbatim, derived index emitted exactly as
`createQueryIndexDDL` writes it). 50k tickets / 5k concepts, each ticket linked
to one concept by `caused-by`, with 10 of 5000 concepts matching the filter.

**The existing derived index does NOT serve a traversal filter, and adding the
neighbour-side one is not by itself enough.** With
`rela_derived_query__concept_status` present, the planner still ignored it and
bitmap-scanned 5000 concepts to find 10 (`Rows Removed by Filter: 4990`).

The cause is that `createQueryIndexDDL` emits a PARTIAL index:

```sql
CREATE INDEX ... ON entities ((properties->>'status'))
  WHERE type = 'concept' AND jsonb_typeof(properties->'status') = 'string';
```

Postgres only matches a partial index when the query implies its WHERE clause.
A traversal emitting `b.type='concept' AND (b.properties->>'status')='rare'`
does not imply the `jsonb_typeof` conjunct, so the index is not a candidate. Add
that conjunct and the index is used immediately:

| query shape | plan | time | buffers |
|---|---|---|---|
| no `jsonb_typeof` guard | Bitmap scan, 4990 rows filtered | 1.725 ms | 334 |
| with `jsonb_typeof` guard | `Index Scan using rela_derived_query__concept_status` | 0.541 ms | 283 |

Fully indexed end-to-end with the guard: neighbour by derived index → edges by
`relations_type_to_idx` → source rows by `entities_type_id_idx` (Heap Fetches: 0).

Scaling check at 250k entities / 250k relations: 3.3 ms, 722 buffers — 5x the
data for ~2.5x buffers, sub-linear, plan shape unchanged.

**Consequences for implementation.** The traversal lowering MUST emit the
`jsonb_typeof(...) = 'string'` conjunct alongside the equality, not merely as a
correctness guard but as the thing that makes the index reachable. This is
already how `propCond` spells a scalar equality
(`internal/store/pgstore/graphquery.go:336`), so the requirement is to reuse
that path rather than hand-roll a comparison — which is an argument for the
`propCond` alias refactor being mandatory rather than cosmetic. The
`PropPredicate.Scalar` flag is what selects that spelling.

Traversal also needs its own derived-index spec on the NEIGHBOUR type.
`store.DerivedObjectSpec` carries a single `Type` and the DDL is partial on
`type = <spec.Type>`, so a traversal condition must contribute a SECOND spec for
the target type — `StaticIndexSpecs` (`internal/queryplan/queryplan.go:134`)
currently derives one spec per query from `sq.EntityTypes[0]` and would need to
emit the traversal's target-type spec too. Without it the join degrades to a
bitmap scan of the whole target type: correct, but linear in the neighbour
population.

Notably this does NOT require a new index SHAPE — the existing
`createQueryIndexDDL` output is exactly right, it just has to be generated for
the traversed-to type as well. That resolves the main risk flagged against the
"SQL is cheap" claim: the SQL is cheap, provided the spec generator is taught to
emit the second spec and the lowering emits the scalar guard.

### The ACL inference channel

This is the sharpest risk and it is NOT the same as the `Any`/`Narrowing`
privilege split.

The row gate is composed **per entity type**: `listPushdown` calls
`provider.ReadQueryFor(ctx, q.Type)` for exactly one type
(`internal/visibility/pushdown.go:73`), and the resulting `GraphQuery`'s relation
predicates carry **no gate of their own**. This is deliberate for the existing
uses — `GraphQuery` documents that "relation predicates walk the graph's identity
structure and are NOT world-resolved: who an entity is related to must not depend
on the reader's world" (`internal/store/graphquery.go:43`).

That reasoning is sound for an ACL-derived predicate over ids the ACL itself
chose. It does NOT transfer to a caller-supplied filter over a neighbour's
PROPERTY VALUES. A traversal filter naively lowered would let a principal write:

```lua
related(entity, 'caused-by', { salary = '100000' })
```

and learn, from which rows come back, the value of a field on an entity they may
not read — or that such an entity exists at all. Both are secrets per
`docs/acl-security.md`: field VALUES are gated by `visible:`, and entity
EXISTENCE is gated at row level ("a hidden entity is nonexistent").

This is an oracle, not a direct disclosure: the traversal returns A-rows, not
B-rows, so redaction of the response never fires. The channel is the
*correlation* between the filter and the result set, and it is a binary search —
repeated queries narrow a hidden value arbitrarily.

The closest solved precedent is neighbour-title leakage on the export/serializer
path, gated by `visibleRelationIDs` (`internal/dataentry/entityserializer.go:178`,
RR-HJV8CP) — batched per page, not per id. That pattern gates ids being SERIALIZED;
it does not gate a predicate being PUSHED DOWN, so it is a model, not a solution.

## Options

### Syntax

**S1. Nested attribute access** — `entity.caused_by.status == 'done'`.
Prettiest, reads as a property path, and `Program.Attributes` extends naturally.
But it offers nowhere to put a type ascription, so it cannot express the union
case — precisely where typing breaks. Viable only for single-target relations.

**S2. Bracket ascription** — `entity.caused_by[ticket].status`. REJECTED: not
valid Lua semantics (`ticket` would be a global lookup), and `walkAttrGet`
rejects any non-string-literal key outright (`internal/predicate/walk.go:31`).

**S3. Fluent chain** — `related(entity,'rel-x'):equal('status','done'):exists()`.
Parses (verified against gopher-lua: the colon form yields
`FuncCallExpr{Method:"exists", Receiver:...}`; the DOT form `.equal(...)` parses
but is semantically wrong Lua — no receiver is passed). Rejected on cost: it
needs a relation-set value kind and method dispatch, which means relaxing both
the record-return guard and `walkCall`'s explicit refusal of `entity.method(args)`
(`internal/predicate/walk.go:335`). Compiling the chain as one rigid static
template avoids that, but then the fluency ADVERTISES a generality the engine
cannot deliver — users will try `:count() > 3` or hold an intermediate in a
variable, and neither works.

**S4. Table form (CHOSEN)** — `related(entity, 'caused-by', { type='ticket', status='done' })`.
Ordinary valid Lua: a call with a string and a table constructor. Every argument
is a static literal, so the whole form lowers without evaluating anything. The
named-args table form ALREADY exists in the walker (`walkTableArg`,
`internal/predicate/walk.go:452`) and is documented there with
`has_relation('x', {status='open'})` as its motivating case — so this is an
extension of an established precedent, not a new mechanism.

Chained hops take a path list:

```lua
related(entity, { 'rel-x', 'rel-y' }, { type='C', prop='foo' })
```

Critically, `related(...)` is **compiled as a statically recognised form**
lowering to a dedicated traversal IR node — NOT a runtime `callNode`. That keeps
`SQLPortable` true, leaves RR-93UN untouched, and preserves `Program.Attributes`
as an exact dependency set.

Known limit: the boolean EXISTS shape puts comparisons INSIDE the table, so
`{ status='done' }` works but ordered comparison needs a nested constructor
(`{ due = { lt = today() } }`) — deferred to v2 alongside ordered pushdown, since
both need the same type-resolution gate.

### Typing the union

**T1. Refuse multi-target hops.** Sound and trivial; rejects a third of the
schema's relations including `depends-on`, `verifies`, `triggered-by`.

**T2. Intersection typing.** A property is referenceable iff every target
declares it with the SAME type. Sound, no new syntax — but per the table above it
excludes `status` on 14 of 15 multi-target relations, i.e. it permits the
traversal and then forbids the only field anyone wants. Arguably worse than T1
because the failure is surprising rather than categorical.

**T3. Required type ascription (CHOSEN).** `{ type='ticket', ... }` resolves the
union to exactly one type, so literal coercion works and the enum is unambiguous.
Lowers to a `b.type = 'ticket'` conjunct — the candidate side already emits
exactly this via `typeArg` in `buildPredicateSQL`. Mandatory when `To` has more
than one element; optional (bare form) when it has exactly one, keeping the 68%
single-target case clean. T2 applies as a bonus where the intersection agrees.

### ACL

**A1. Gate the traversed type with its own ReadQuery (CHOSEN).** Compose
`ReadQueryFor(ctx, targetType)` for each hop and AND it into that hop's EXISTS
body, so the traversal can only match neighbours the principal may READ. Cost is
one extra ACL composition per hop at query-build time (not per row), and it
composes naturally because the result is another set of predicates on the same
joined table. `DenyAll` on a hop makes the whole traversal match nothing;
`AllowAll` contributes no conjunct.

**A2. Restrict filterable properties to non-`visible:`-gated ones.** Necessary
IN ADDITION to A1, and independent of it: A1 gates row EXISTENCE, but a field
readable only under a conditional `visible:` grant is still a value secret on a
row the principal CAN see. A property whose visibility is conditional must not be
filterable, or the oracle survives the row gate.

**A3. Forbid traversal on ACL-evaluated surfaces entirely.** The blunt option:
allow it in view/list `where:` but not in `visible:`/affordance `when:`. Avoids
the chicken-and-egg where evaluating a grant needs a read gate that the grant
itself produces. Probably correct for v1 regardless of A1/A2, because affordance
predicates run INSIDE the gating decision.

Note the `Any`/`Narrowing` split is a SEPARATE requirement, not an alternative: a
caller-supplied traversal must lower into `GraphQuery.Narrowing`, never `Any`.
Those are deliberately different types so the mistake cannot compile
(`internal/store/graphquery.go:77`).


### Implementation findings (store + ACL layers landed)

Three constraints surfaced while building the gate that the design above did
not anticipate. All three are fail-CLOSED refusals rather than silent
degradations, because each is a case where the predicate cannot express what
the policy means:

1. **A face-restricted read cannot be gated.** `store.EndpointPredicate` has no
   face field, so when `ReadQueryResult.Faces` is non-empty the gate refuses.
   Emitting the predicate anyway would traverse through content states the
   principal may not read.
2. **A disjunctive (`Any`) authorization ceiling cannot be gated.** The
   conferred-role path produces `GraphQuery.Any` branches; an EndpointPredicate
   has no OR field, so flattening would either widen (escalation) or misstate
   the ceiling. Refused.
3. **The field gate is POLICY-WIDE, not per-principal.** `ConditionallyVisible`
   asks whether ANY role's `visible:` grant for the type/property carries a
   `when:`. A per-principal answer would make the set of filterable properties
   vary by role, so the privileged caller becomes an oracle for the
   unprivileged one — and it is what lets the refusal be a load-time error
   rather than a request-time one.

Also settled: a **dangling edge is not a match**. This falls out of the INNER
JOIN in SQL, and the Go path was made to agree explicitly rather than by
accident; it is pinned by a conformance test.

The security tests were verified to FAIL when the row gate is removed, and the
EXPLAIN test to fail when the endpoint comparison is hand-rolled without the
scalar guard. A security test that cannot fail is worth nothing.

## Recommendation

Ship `related(entity, path, {constraints})` — table form (S4), required type
ascription on unions (T3), lowered statically to a nested predicate on
`store.RelationPredicate`, gated by A1 + A2 + A3.

Scope for v1:

- ANY semantics (EXISTS); no "all"
- equality/inequality only; ordered comparison deferred with the nested-operator
  table form
- chaining allowed, depth capped at compile time against `DepthCap`
- **refuse to compile** when a neighbour property is not pushdown-eligible,
  rather than silently falling back to Go per-row evaluation

That last point matters most. A silent Go fallback is how this feature becomes
the N+1 that the collection-reads rule exists to prevent, and it would be
invisible to `storetest.Counting` budget tests because it happens above the
store's query path. Failing at config load, naming the property, matches how
`condition:` compile failures are already treated as load errors.

### Work breakdown

| Area | Change | Risk |
|---|---|---|
| `internal/predicate` | Traversal IR node + static recognition of the form | Low — table-arg walking exists |
| `internal/store` | Nested predicate field on `RelationPredicate` | Low |
| `pgstore` | Join + conjuncts in EXISTS; `propCond` needs an alias param (hardcodes `e.`, `graphquery.go:331`) | Medium — shared with `visiblesearch.go` |
| `graphquerynaive`, `sqlitestore` | Backend parity, must pass `storetest` | Medium |
| `queryplan` | Eligibility gate resolving the TARGET type's property | High — new logic, no existing helper |
| `visibility`/`acl` | Per-hop ReadQuery composition (A1), filterability rule (A2) | High — security-critical |
| `queryplan` | SECOND derived-index spec for the traversed-to type | Medium — settled, see above |
| EXPLAIN tests | Pin the guard-implies-partial-index behaviour | Low — shape verified |

### Open questions

1. ~~Index shape is unverified.~~ **SETTLED** — measured on PG 18.6; see
   "Index behaviour is settled" above. No new index shape is needed, but the
   lowering must emit the `jsonb_typeof` scalar guard (or the partial index is
   not matched) and `StaticIndexSpecs` must emit a second spec for the
   traversed-to type.
2. **No metamodel helper exists** for "properties common to a set of types with
   identical declared type." `GetRelationTo` exists
   (`internal/metamodel/schema_output.go:106`); the intersection logic is new
   code. This plus the union rules is the real new work, not the SQL.
3. **Does the frontend need this?** If view/kanban `where:` surfaces must support
   traversal, that forces the pushdown path and rules out any Go-evaluated
   fallback permanently.
