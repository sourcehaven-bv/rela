---
id: RES-05JD73
type: research
title: Pushing date conditions into the store query layer
status: done
summary: 'Push date conditions down in two steps; gated on making next-action Query accept expressions first (today it is search syntax, so no date condition is expressible there at all). Step 1: constant-fold today() at plan time and push ordered comparisons - ISO-8601 dates sort lexicographically as text, so the store needs no metamodel and the existing never-widen pushdown contract holds. Step 2: add a DerivedIndex reconciler rule reusing the TKT-3Q0GP1 machinery, which already emits partial expression indexes over properties->>''x''. Reject query-inferred indexes: DDL should follow a reviewable declaration, not traffic. rrule_next never pushes down.'
---

## Problem

Date conditions (`days_between`, `date_add`, `rrule_next` — TKT-HQONQE,
TKT-8GD41J) are evaluated **in Go, per entity, after loading**. That is fine
where the candidate set is one entity (automation on a write) or already in
memory (validation), but wrong for whole-graph queries.

**Next-actions is the case that motivates this.** A global next-action source
runs its `Query` over the entire graph, on the dashboard, per principal, per
page load (`dataentry/nextaction.go:46` → `queryCandidates` → `executeQuery`).
Date-filtered next-actions are the obvious use ("what's due this week?") and are
currently unexpressible — `Query` takes *search* syntax (`type:task
prop:status=doing`), not the predicate dialect.

The failure mode is the one that sneaks up: fine at tens of records, slightly
slower at hundreds, and by the time anyone notices it is a load-bearing
dashboard doing a full scan. Worth designing before it is live, not after.

## Context

### The pushdown seam already exists

`executeQuery` (`internal/dataentry/helpers.go:399`) already does partial
pushdown, and its comment states the safety property:

> Push the equality filters into the store as a PRE-FILTER, cutting the rows
> loaded. The Go pass below still evaluates every filter, including the pushed
> ones, and remains authoritative.

The belt-and-braces is deliberate: `store.PropPredicate` compares by STRING
form, `filter.MatchAll` is metamodel-aware, and on a typed property **they
disagree** (`count!=03` against integer 3 is a non-match typed, a match as
strings). Keeping both means the store can only ever REMOVE rows the Go pass
would also remove — never widen. Any date pushdown must inherit that property.

### What blocks ordered comparison

`store.PropPredicate` supports only `=` and `!=`, and `graphquery.go:32-37` says
why:

> Deliberately only equality and its negation: ordered comparison
> (`due < 2026-01-01`) needs the property's declared type from the metamodel to
> avoid comparing dates lexicographically, and the store layer does not consult
> the metamodel.

The objection is real but **narrower than it looks for dates specifically**:
ISO-8601 `YYYY-MM-DD` sorts lexicographically exactly as it sorts calendrically.
So a text comparison on `properties->>'due'` is already correct for dates — no
cast, no date parsing, no metamodel knowledge inside the store. The typing
decision stays in `pushdownPrefilters`, which already receives `svc.Meta`.

### `today()` folds at plan time

`today()` is caller-injected and pinned per evaluation
(`NewEvaluatorWithClock`), so date expressions reduce to a comparison against a
literal BEFORE the store is involved:

```text
days_between(entity.due, today()) <= 7    ->  due <= '2026-08-26'
date_add(entity.due, 3, 'day') < today()  ->  due <  '2026-08-16'
```

This is constant folding, not date arithmetic in SQL. It is the whole trick.

### The index machinery landed yesterday

**TKT-3Q0GP1** (PR #1371, commit `9876f0be`) shipped
`store.DerivedSchemaReconciler` — pgstore synthesizes Postgres objects from the
metamodel. The `unique: true` DDL is *exactly* the shape a date index needs:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS rela_derived_uniq__<hash>
  ON entities (type, (properties->>'email'))
  WHERE type = 'persoon' AND properties->>'email' <> '' AND properties->>'email' IS NOT NULL
```

A partial EXPRESSION index over `properties->>'<prop>'`, scoped by `type`. The
hard parts are solved and generic:

- **Stateless reconciliation** — metamodel is desired, `pg_indexes` is actual,
a name prefix marks ownership. No persisted state to drift.
- **Deterministic naming** — SHA-256 truncated to Postgres's 63-byte limit,
NUL-separated so `("ab","c")` and `("a","bc")` cannot collide.
- **Advisory lock** (`"RELD"`), distinct from migrate/write/sweep.
- **DDL-injection defense** — `safeDDLName` duplicated rather than importing
metamodel (arch-lint boundary).
- **`DerivedObjectKind`** exists precisely so rules can be added; the doc
anticipates it: *"a future rule (e.g. an enum CHECK under a different prefix)
cannot be clobbered by this one."*
- **`DerivedUnenforced`** already models "declared but cannot be created" —
the state a non-unique index would rarely need but a unique one does.

So a date/range index is **a second rule under a new prefix**, not new
machinery.

### Constraints

- `internal/store` may not import `internal/metamodel` (arch-lint). Typing must
stay caller-side.
- Any new `store.Store` behaviour must pass `internal/store/storetest`
(CLAUDE.md) — ordered operators would need conformance cases across
fsstore/memstore/pgstore, OR be an optional capability like the reconciler.
- Reconciler DDL is NOT `CONCURRENTLY` (it runs inside the migrate
transaction); index build blocks writes on a large table.
- Empty/missing property semantics differ between `propmatch` (store) and the
expression engine (`nil`). A pushed predicate must not exclude rows the Go pass
would keep — this is the exact trap the TKT-8GD41J review surfaced.

## Options

### A — Do nothing; keep date filtering in Go

- **Pros**: zero work; correctness already proven; no store-layer churn.
- **Cons**: whole-graph next-action sources load every candidate row and filter
in memory. Degrades silently with graph size — the stated worry.
- **Effort**: none.

### B — Constant-fold + push ordered comparisons, no new indexes

Add ordered ops to `PropPredicate` (or a parallel typed predicate), fold
`today()` at plan time, emit `properties->>'due' <= '2026-08-26'`. Postgres does
a seq scan but the *rows loaded into Go* drop to the matching set.

- **Pros**: most of the I/O win for a fraction of the work; no DDL, no
reconciler rule, no operator-facing config. Works on every backend that can
compare strings.
- **Cons**: still a seq scan in postgres; fsstore/memstore gain little (they
already hold everything in memory). Needs `storetest` conformance for the new
ops.
- **Effort**: M.

### C — B, plus a `DerivedIndex` reconciler rule

As B, then add a second reconciler rule emitting a non-unique partial expression
index (`rela_derived_idx__<hash>`) for properties declared indexable.

- **Pros**: turns the seq scan into an index scan; reuses reconciler machinery
almost wholesale; the operator declares intent in one place.
- **Cons**: needs a metamodel field (`indexed: true`? or infer from
`unique`/`date` type?) — a new operator-facing knob with a naming and defaults
decision. Index build blocks writes; on a large existing table the first
reconcile is a stall. Wasted indexes if declared and unqueried.
- **Effort**: M on top of B (the reconciler is the easy half).

### D — Infer indexes from observed queries

Track which `properties->>'x'` comparisons actually run and create indexes for
the hot ones.

- **Pros**: no operator knob; indexes match real usage.
- **Cons**: implicit, non-reproducible, and DDL driven by traffic is a
production surprise generator. Contrary to the reconciler's stateless
desired-vs-actual design, which is what makes it safe.
- **Effort**: L, and **not recommended** — noted to be explicitly rejected.

## Recommendation

**Sequence B, then C — but only after date conditions are reachable from a
next-action `Query`.**

The ordering is forced by dependency, and each step is independently useful:

1. **Make next-action `Query` accept a predicate expression.** Today it is
search syntax, so no date condition can be written there at all. Until this
exists, pushdown optimizes a query nobody can express. *This is the real next
ticket.*
2. **Option B** — fold `today()` at plan time, push ordered comparisons. Cuts
rows loaded, which is the dominant cost on a dashboard scan, and is
backend-agnostic.
3. **Option C** — add the `DerivedIndex` rule once there is a measurable scan
worth indexing. Cheap by then, since the reconciler exists.

**Do not push `rrule_next`.** No index evaluates an RRULE; it stays a
post-filter, and any pushdown must split the condition into a pushable conjunct
plus a residual rather than refusing the whole expression.

**Reject option D** for the reason the reconciler is safe: statelessness. DDL
should follow a declaration someone wrote and can review, not accumulated
traffic.

### Open questions for the design ticket

- **How is an index declared?** `indexed: true` on `PropertyDef` is the obvious
shape and mirrors `unique: true`. Alternative: infer for `date`-typed
properties, since those are the ones range-scanned — fewer knobs, but surprising
and possibly wasteful.
- **Optional capability or `store.Store` contract?** The reconciler set the
precedent of an optional, type-asserted capability for something only pgstore
can do. Ordered comparison is different — every backend *can* compare strings —
so it likely belongs in the contract, with `storetest` cases.
- **First-reconcile stall.** `CREATE INDEX` is not `CONCURRENTLY` and runs in
the migrate transaction. Acceptable for `unique` (correctness); a performance
index may want `CONCURRENTLY` outside the transaction, which changes the
reconciler's transactional shape.
- **Is there a measured problem yet?** Not live today. The case for building
ahead is that this degrades invisibly; the case against is speculative
optimization. A benchmark over a synthetic large graph would settle whether B
alone suffices.
