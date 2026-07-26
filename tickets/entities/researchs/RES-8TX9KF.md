---
id: RES-8TX9KF
type: research
title: 'Effective-access map: enumerate who can access what, with provenance and drift detection'
summary: 'ACL effective-access map: enumerate who-can-access-what with all-routes provenance, filtering, drift. UC3 (who-can) SHIPPED in TKT-9089I6.'
status: done
---

## Implementation notes — UC3 (`who-can`) SHIPPED (TKT-9089I6, PR #1141, merged 2026-07-16)

The first slice — `rela acl who-can <verb> <entity>` — is in `develop`. Package
`internal/aclmap` + `acl.Request.AccessRoutes` + `Declarative.EveryoneGrants`.
What implementation and the two review rounds actually taught us (these override
/ refine the design assumptions above):

- **Read has no attribution today — route it through the real read path.**
There is no `OpRead`; `computeForEntity` only attributes writes. Reconstructing
read from `computeForEntity` is a *parallel reimplementation* of `readQuery`
(separate BFS + edge-orientation) that can produce a **false negative** — the
worst failure for an attestation tool. FIX (design review, RR-7UXWNA): read is
gated on the real `PermitsRead` boolean; the attributions only *explain* an
already-confirmed grant. The durable guard is a **two-way conformance test**
(`who-can read E` ⟺ `PermitsRead(E)`, over the full candidate universe, all
verbs) — it is the single most valuable test in the slice.

- **Do NOT exclude "groups" by graph topology.** The design assumed group
entities (membership-relation *targets*) should be hidden as non-actors. That is
WRONG: "is a membership target" ≠ "is a non-actor container." A manager who has
reports is a membership target *and* a real principal; excluding them dropped a
genuine grantee (code review, RR-C5Q743 — a real false negative caught only by
the cranky reviewer, not the design review). SHIPPED behavior: **exclude nothing
by topology; report every entity the runtime would grant, each row's routes
making a group entity easy to recognize.** The two-way conformance test is what
makes "report exactly the runtime's set" safe.

- **`member-of` is transitive** — `PERS-REPORT → PERS-BOSS → ROLE-EXEC` confers
ROLE-EXEC's role on the report. The map reports this (correctly); it is not a
false positive. (My initial C1 test asserted otherwise and was wrong; the
conformance-against-runtime approach corrected the mental model.)

- **Merge by effective principal.** With `principal_property`, one human surfaces
as several candidate keys (raw UPN + resolved entity). Emit ONE row per
effective principal with unioned routes (RR-XC2NTO), deterministic ordering
(RR-2NZRXO). This matters because the drift/diff consumer (UC8) diffs this
output — duplicate rows corrupt the artifact.

- **Provenance depth shipped = terminal facts** (kind + group/ancestor/relation
*names*), NOT full hop-by-hop chains. Terminal facts already name the edge to
cut, so revocation is actionable. The JSON reserves a list-of-route-objects
shape so a later `hops[]` field is non-breaking. **Full chains are a
fast-follow**, deferred with the `Source`-restructure it requires.

- **Cost:** O(principals) fresh resolver Requests per query (no cross-principal
cache); bounded by `depthCap=5`, fine for single-entity `who-can`. A reverse
principal→entity index is the optimization the `map` command will need
(RR-K72ML0, deferred).

**Still open (this feature):** `map` (UC1/UC2), `can`, full hop-chains, drift
(UC8), assertions (UC4/UC5), reverse index, web view — see FEAT-RCQ6SJ.

## Context

### The ACL model (as it exists)

- **Policy**: declarative `acl.yaml` at project root (never in the metamodel — arch-lint enforced). `internal/acl/policy.go:77-85`.
- **Principal**: `principal.Principal{User, Tool, RawUser}` — `internal/principal/principal.go:40-44`. Roles/groups are **not** stored on the principal; they are *derived* at request time from `acl.yaml` assignments + the graph.
- **Decision function**: `acl.ACL.AuthorizeWrite(ctx, WriteRequest) Decision` — `internal/acl/acl.go:61-63`. `Decision` carries `Attributions []RoleAttribution` — every allow/deny names the rule and role that fired.
- **Read path**: `Request.ReadQuery(ctx, entityType)` → `{AllowAll | DenyAll | Query}` (`internal/acl/readquery.go:27`); per-entity `Request.PermitsRead` is the one-shot gate the who-can tool now drives read decisions through. `search.VisibleSearcher.SearchVisible` with a per-type `TypeScope` map (`internal/search/types.go:105`), fail-closed.
- **Resolver / provenance**: `computeGlobals` (`internal/acl/resolver.go:15`) walks `member-of` closure; `computeForEntity` (`resolver.go:118`) crosses the member set × the entity's ancestor chain (`inherit_roles_through`) against `role_relations`. Provenance is the **`Source` taxonomy**: `Global`, `Group`, `Local`, `LocalViaGroup`, `LocalViaAncestor`, `LocalViaGroupAndAncestor` — `internal/acl/source.go:8-15`, documented in `docs-project/entities/concepts/CON-authorization.md:53-63`.

### The principal universe (net-new enumeration — SHIPPED in aclmap)

No existing code enumerated all principals — the resolver is always pull-based
from one known `principal.User`. The map's union (implemented in
`internal/aclmap/enumerate.go`):

```
(all entities of user_entity_type)         # resolvable human principals; principal_property is unique
∪ (all keys in assignments)                # direct + group assignment keys
∪ (all sources of membership_relation edges)  # anything reachable in a member-of closure
∪ (all sources of role_relations edges)    # holders of role-conferring edges
```

Plus the built-in **`everyone`** role (`EveryoneRole`, `policy.go:33`) —
surfaced as a single global entry, never per-principal. Nothing is excluded by
topology.

### Constraints

- **Scope caveat (important):** `principal_property` resolution is wired into **data-entry transport only** (`internal/acl/declarative.go:112-126`). CLI/MCP/scheduler/desktop authorize against the *raw* principal. An entity-keyed map reflects data-entry-transport access; the tool documents this in `--help`.
- **Read-path purity:** the map is built from declarative policy + graph reachability only — no user-supplied Lua on the read path.
- **Consumer-side interfaces:** the map engine takes narrow interfaces (`EntitySource` for GetEntity/ListEntities/ListRelations, a `Resolver` for ForPrincipal/PermitsRead/EveryoneGrants/Policy), not a service locator.
- **CLI surface:** extends the existing `rela acl` command family — `internal/cli/acl.go`.
- **Scale:** the `map` command (not yet built) must aggregate by entity type + list only per-entity exceptions; `who-can` is single-entity so it doesn't hit O(P·E).
- **Future web surface:** engine emits stable, versioned JSON so a data-entry web view can consume it later without re-deriving.

## Options

### Option A — Inventory engine + query/filter CLI (recommended v1) — PARTIALLY SHIPPED (who-can done; map/can pending)

A new package (`internal/aclmap`) that enumerates the principal universe, and
for each principal computes effective access, aggregated by entity type with
per-entity exceptions, every row carrying its all-routes `Source` provenance.
Exposed as:

```
rela acl map [--principal P] [--type T] [--entity E]
             [--verb read|create|update|delete]
             [--via global|group|relation|inheritance]
             [--format text|json]
rela acl can <principal> <verb> <entity|type>   # yes/no + reason, exit code (UC1)
rela acl who-can <verb> <type|entity>           # list principals + provenance (UC3/UC5) — SHIPPED
```

- **Covers:** UC1, UC2, UC3 (shipped), UC6 (via two runs), UC7 (partially).
- **Pros:** reuses existing resolver primitives; no new policy syntax; is the shared foundation every other mode builds on; the exact engine a web view needs; provenance-first (audit, not just a grid).
- **Cons:** O(P·E) worst case for `map` — mitigated by type-aggregation + exceptions; entity-keyed map has the data-entry-transport caveat.
- **Effort:** M.

### Option B — Conformance assertions (`rela acl verify`)

An expectations file (`acl-expectations.yaml`) of positive/negative assertions
(`expect: alice can read ticket`; `expect: contractor cannot delete *`), checked
against the engine; non-zero exit on any failure.

- **Covers:** UC4 (regression, by encoding the invariants that matter), UC5 (least-privilege gates — negative assertions, which a snapshot cannot express).
- **Pros:** encodes *intent*, degrades gracefully (only asserted invariants are gated), CI-friendly, lives next to `acl.yaml`.
- **Cons:** a spec to maintain; format needs design (defer until the engine exists and real output shapes it).
- **Effort:** S–M on top of A. **Layered as v2.**

### Option C — Drift detection (snapshot time-series)

Persist the Option-A map as a stable-shaped JSON snapshot on a schedule (`rela
scheduler` daily); diff current vs. previous; emit per-principal gained/lost
deltas and flag large swings.

- **Covers:** UC8. Reuses Option A's JSON as the snapshot format.
- **Pros:** turns the snapshot from a *spec you maintain* into an *automatic time-series*; composes with the scheduler + append-only audit log; no expectations file.
- **Cons:** needs a snapshot store + diff/magnitude thresholds; noisy on legitimately large changes (needs a swing threshold).
- **Effort:** M on top of A. **v2/v3.**

### Option D — Extend aclaudit into Tier C/D directly

Grow `internal/aclaudit` with graph-aware (Tier C) and reachability (Tier D)
checks, emitting the same `Finding` shape.

- **Covers:** UC7 fully, parts of UC4/UC5 as heuristics (over-provisioning only).
- **Pros:** reuses the finding/severity/CLI machinery; matches the reserved-tier design intent; heuristics need no expectations file.
- **Cons:** aclaudit's `Finding` model is lint-shaped, not inventory-shaped. Heuristics only catch *over*-provisioning, never "access missing where needed."
- **Effort:** S per check; does not deliver the map itself.

## Recommendation

**Ship Option A first** — DONE for `who-can` (UC3). Next: `map --principal`
(UC1/UC2) and `can`, then the deferred fast-follows. Fold Option D's cheap
graph-aware heuristics into `rela acl audit` opportunistically.

Then layer, in order of value:
- **Option C (drift, UC8)** — reuses A's JSON as the snapshot format, composes with the scheduler.
- **Option B (assertions, UC4/UC5)** — the only mechanism expressing "access **where needed**" (under-provisioning) and negative least-privilege invariants.

**Tradeoffs accepted:** (1) the entity-keyed map reflects data-entry-transport
access due to the `principal_property` resolution caveat — documented in
`--help`, not hidden; (2) O(P·E) cost mitigated by type-aggregation + exceptions
in the future `map`; (3) `everyone` is surfaced as a single global entry so
anonymous/unbounded access is never silently dropped; (4) **provenance is
all-routes** (terminal facts shipped, full hop-chains a non-breaking
fast-follow); (5) **nothing is excluded by topology** — the report is exactly
the runtime's grant set, guarded by a two-way conformance test, so no real actor
is ever dropped.
