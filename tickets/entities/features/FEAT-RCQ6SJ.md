---
id: FEAT-RCQ6SJ
type: feature
title: ACL effective-access map (rela acl map / can / who-can)
summary: rela acl map/can/who-can — enumerate who can access what across the graph, each grant showing every route it was acquired by.
description: A CLI that materializes the effective-access relation (principal × entity × verb) with full-chain, all-routes provenance, so operators can verify ACL is configured to allow access where needed and deny it where not — without manual per-principal, per-entity checks. v1 = inventory engine + map/can/who-can. Grounded by RES-8TX9KF.
priority: medium
status: proposed
---

# ACL effective-access map — v1 (inventory engine)

Grounded by **[[RES-8TX9KF]]** (`researches` → `authorization`, `audit-log`).
This is v1 = **Option A** only (inventory engine + `map`/`can`/`who-can`). Drift
(UC8), conformance assertions (UC4/UC5), and the web view are explicitly **out
of scope** here — separate follow-up tickets — but the v1 JSON schema is
designed so they can consume it unchanged.

## Problem

rela can answer "can P do X to E right now?" (per-request) and lint the static
policy (`rela acl audit`). It cannot answer **"who can access what across the
whole graph, and by which route?"** — the cross-product view an operator needs
to verify access is granted where needed and denied where not. `aclaudit`
reserves this as Tier C/D "future work" (`internal/aclaudit/aclaudit.go:20-21`).

## Scope (v1)

Three subcommands under the existing `rela acl` family (`internal/cli/acl.go`):

```
rela acl map [--principal P] [--type T] [--entity E]
             [--verb read|create|update|delete]
             [--via global|group|relation|inheritance]
             [--format text|json]
rela acl can <principal> <verb> <entity|type>   # yes/no + reason, exit code
rela acl who-can <verb> <type|entity>           # principals + provenance
```

Backed by a new package **`internal/aclmap`** that takes narrow consumer-side
interfaces (`ListEntities`, a `ForPrincipal`-style resolver, policy accessors) —
**not** a service locator (CLAUDE.md). Reuses existing resolver primitives
(`Declarative.ForPrincipal` → `computeForEntity`, the `Source` taxonomy); adds
**no** new policy syntax and **no** read-path Lua.

## Use-cases covered (from RES-8TX9KF)

| UC | What v1 delivers |
|----|------------------|
| UC1 onboarding | `map --principal` — per-principal verb grid, spot a stray write grant |
| UC2 offboarding | `map --principal` — complete reachable set, `everyone`-only = cut off |
| UC3 sensitive spot-check | `who-can read <entity>` — readers + the path each took |
| UC6 blast-radius | `map --principal` before/after diff — count of newly-reachable entities |
| UC7 (partial) | surfaces everyone-readable types etc.; full heuristics are a separate aclaudit ticket |

## Acceptance criteria

### Enumeration
- [ ] Principal universe = `(entities of user_entity_type)` ∪ `(assignment keys)`
∪ `(sources of membership_relation edges)` ∪ `(sources of role_relations
edges)`. Union is deduplicated and stable-ordered.
- [ ] The built-in `everyone` role is surfaced as an explicit pseudo-principal
row, never silently omitted.
- [ ] Works with and without `principal_property` configured. When configured,
resolved-entity IDs are used and the raw value shown alongside (`PERS-BOB
(bob@…)`); when not, raw principal strings are used.

### Provenance (hard requirement — see RES-8TX9KF "Provenance requirement")
- [ ] Every grant shows its **full edge chain**, every hop named:
`PERS-DAVE → editor-of → FOLDER-Q3 → belongs-to ⤳ INC-042 → editor`. Never a
bare label like "via inheritance".
- [ ] When a permission is granted by **multiple routes**, **all** routes are
listed (not the shortest). Redundant access must be visible so "fully revoked"
means every path removed.
- [ ] All six `Source` kinds render correctly: `Global`, `Group`, `Local`,
`LocalViaGroup`, `LocalViaAncestor`, `LocalViaGroupAndAncestor`.

### Scale / output shape
- [ ] Default output aggregates by **entity type** (type baseline row per
principal), then lists only **per-entity exceptions** where graph/ inheritance
makes an entity differ from its type baseline. Does not print a row per entity
for thousands of entities.
- [ ] A **headline summary line/count** is emitted (e.g. grant-source count,
newly-reachable count for a diff) — required by UC6 (blast-radius reads a count,
not rows).
- [ ] `who-can` on an entity with N readers prints N lines, each with its
provenance chain(s).

### JSON schema (forward-compatible with drift + web)
- [ ] `--format json` emits a **versioned, stable** schema.
- [ ] Per grant, provenance is an **ordered list of route objects**, each an
**ordered list of `{entity, relation}` hops** ending in a role — not a single
enum. (Consumed unchanged by the future drift + web tickets.)
- [ ] JSON output is deterministic (sorted keys, stable ordering) so it diffs
cleanly.

### `can` (spot-check)
- [ ] `rela acl can <principal> <verb> <entity|type>` prints allow/deny + the
reason (the deciding route, or "no grant") and sets a **non-zero exit code on
deny** so it is scriptable.

### Correctness / tests
- [ ] Golden tests against the RES-8TX9KF grounding policy (ISMS-style
`acl.yaml`: `security` group, `incident` type, `editor-of`/`responds-to`
role-relations, `belongs-to` inheritance) covering all six route kinds and a
multi-route (redundant) grant.
- [ ] Matches the resolver's actual decisions — the map's "why" is the same
"why" the runtime enforced (cross-check against `AuthorizeWrite` /
`computeForEntity` for a sample set).
- [ ] Package coverage meets its `.testcoverage.yml` floor; `just arch-lint`
passes (no new boundary violations); `just plimsoll` passes.

## Documented tradeoffs / caveats (carry into `--help` and docs)
- [ ] **Data-entry-transport caveat:** `principal_property` resolution is wired
into data-entry only (`internal/acl/declarative.go:112-126`); CLI/MCP/ scheduler
authorize against the raw principal. The entity-keyed map reflects
data-entry-transport access — documented in `--help`/docs, not implied to be
universal.
- [ ] O(P·E) cost is mitigated by type-aggregation + exceptions, not
eliminated; note this for very large graphs.

## Explicitly out of scope (follow-up tickets)
- Drift detection (`rela acl diff --alert-swing`) — UC8, reuses this JSON.
- Conformance assertions (`rela acl verify` + expectations file) — UC4/UC5.
- Graph-aware heuristics in `rela acl audit` — UC7 full.
- `--what-if` edge simulation — UC6 advanced.
- Data-entry web view — consumes this engine's JSON.
