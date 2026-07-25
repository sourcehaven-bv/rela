---
id: TKT-B1YE7
type: ticket
title: rela acl map --principal <P> — per-principal effective-access view (UC1/UC2)
kind: enhancement
priority: high
effort: m
status: in-progress
---

# `rela acl map --principal <P>` — per-principal effective access

Second slice of **[[FEAT-RCQ6SJ]]** (ACL effective-access map), building on the
shipped UC3 engine (`internal/aclmap`, [[TKT-9089I6]]). Delivers **UC1
(onboarding verification)** and **UC2 (offboarding / contractor review)** from
[[RES-8TX9KF]]: fix the *principal*, show everything they can reach and by what
route — the inverse of `who-can` (which fixes the entity).

## Command

```
rela acl map --principal <P> [--verb read|create|update|delete]
                             [--type <T>] [--format text|json]
```

Output: for principal `P`, their effective access **aggregated by entity type**
(the type baseline), with **per-entity exceptions** listed below the baseline
where graph/inheritance makes one entity differ. Each grant carries its
all-routes provenance (terminal facts), same as `who-can`.

## Why this is mostly projection, not new machinery

The UC3 engine already has the load-bearing pieces:
- **`accessFor(raw, verb, entityType, entityID)`** (`whocan.go`) — computes one
principal's routes for one (entity, verb). `map --principal` fixes the principal
and iterates entities instead of iterating principals.
- **Read fidelity** — `AccessRoutes` gates read on `PermitsRead`; reuse verbatim
(the anti-false-negative guarantee carries over).
- **Merge / provenance / JSON route shape** — reuse.

New work:
1. **`Engine.MapPrincipal(ctx, principal, verbs)`** — for the given principal,
walk the entity space and compute access per (entity, verb). Uses
`PermitsReadMany` / batched reads where possible.
2. **Type-aggregation + exceptions** — the output shape from RES-8TX9KF
(constraint): a per-type baseline row, then only the entities whose access
differs from that baseline (via a role-relation edge or inheritance). This is
the scale mechanism — do NOT print a row per entity for thousands of entities.
3. **Headline summary** — grant-source count + "N write grants outside
expected" so UC1's "did the group grant more than intended?" reads at a glance,
and UC2's "everyone-only = fully cut off" is a one-line signal.

## Cost — the reverse-index question ([[RR-K72ML0]] from UC3, now due)

`who-can` deferred the reverse principal→entity index because single-entity
queries don't hit O(P·E). `map --principal` iterates the entity space for ONE
principal — O(E) per run, with the resolver's member-of walk amortized across a
single `Request` (already memoized). This is acceptable without a reverse index;
the reverse index only becomes necessary for the whole-graph `map` (no
`--principal`), which is a LATER slice. Confirm during planning that a single
principal's O(E) is fine at target scale; if not, scope the index in.

## Acceptance criteria

- [ ] `rela acl map --principal P` lists P's effective access, aggregated by
entity type, with per-entity exceptions below each type baseline.
- [ ] Read decided by the runtime read path (reuse UC3's `AccessRoutes`);
the read-vs-runtime conformance guarantee holds for the per-principal view too
(a test asserts it).
- [ ] `--verb` filters to one verb; `--type` filters to one entity type;
absent = all verbs / all types.
- [ ] Each grant shows all routes with terminal-fact provenance, same shape as
`who-can`.
- [ ] A **headline summary line** (grant-source count; a distinguishable
"`everyone`-baseline only" state for UC2 cut-off verification).
- [ ] `--format json` reuses the versioned schema (extended, not forked, from
the who-can `Route`/`PrincipalAccess` shapes); deterministic ordering.
- [ ] `principal_property` resolution + the data-entry-transport caveat carried
over and documented in `--help`.
- [ ] Engine method in `internal/aclmap` taking the existing narrow interfaces;
no service locator. CLI command under `rela acl` (`*readServices`).

## Tests

- [ ] UC1 scenario: a principal freshly added to a group shows exactly the
group's verbs and no stray write grant (the "did the grant over-reach?" check).
- [ ] UC2 scenario: a principal with only the `everyone` baseline renders as the
cut-off state; a lingering `editor-of`/group grant shows as a named row.
- [ ] Per-entity exception surfaces (an entity reachable via inheritance that
differs from its type baseline is listed; identical-to-baseline entities are NOT
enumerated).
- [ ] Read-vs-runtime conformance for the per-principal view.
- [ ] `just arch-lint`, `just plimsoll`, coverage floors.

## Explicitly deferred (later slices)
- Whole-graph `map` (no `--principal`) + the reverse principal→entity index.
- `rela acl can` (yes/no spot-check) — trivial, can bundle here or separately.
- Full hop-by-hop provenance chains.
- Drift (UC8), conformance assertions (UC4/UC5), web view.
