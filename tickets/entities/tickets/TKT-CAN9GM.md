---
id: TKT-CAN9GM
type: ticket
title: rela acl can <P> <verb> <E> + whole-graph map (no --principal) — spot-check & full inventory (UC6/UC7)
kind: enhancement
priority: medium
effort: m
status: done
---

# `rela acl can` + whole-graph `rela acl map`

Third slice of **[[FEAT-RCQ6SJ]]** (ACL effective-access map), building on the
shipped UC3 engine (`internal/aclmap`, [[TKT-9089I6]]) and the per-principal
`map` (`MapPrincipal`, [[TKT-B1YE7]]). Delivers the two remaining v1 inventory
surfaces from [[RES-8TX9KF]] Option A:

- **`rela acl can <principal> <verb> <entity>`** — a scriptable yes/no
  spot-check with a non-zero exit on deny. Nearly free: one `AccessRoutes`
  call, no enumeration.
- **`rela acl map` (no `--principal`)** — the *whole-graph* inventory: every
  principal's effective access, aggregated by type with per-entity exceptions.
  This is the slice that finally makes the **reverse principal→entity index**
  ([[RR-K72ML0]], deferred through the last two slices) load-bearing, because it
  is genuinely O(P·E).

## Commands

```
rela acl can <principal> <verb> <entity>   [--format text|json]
rela acl map [--verb read|create|update|delete] [--type <T>] [--format text|json]
```

`can` is a leaf spot-check (fixes principal AND entity AND verb → one boolean +
deciding route(s)). Whole-graph `map` is `MapPrincipal` fanned across the
enumerated principal universe — the same per-type-baseline / per-entity-exception
shape, grouped per principal.

## Why `can` is trivial and `map` is not

**`can`** reuses `accessFor` verbatim: resolve the principal, call
`AccessRoutes(verb, type, entityID)`, print allow + the route(s) or deny + "no
grant", set exit code. Read fidelity carries over (gated on `PermitsRead`). No
new engine machinery; it is the single-cell case of both existing commands.

**Whole-graph `map`** is where cost bites. `who-can` fixed the entity (O(P));
`map --principal` fixed the principal (O(E), member-of walk amortized per
`Request`). Whole-graph fixes neither: O(P) principals × O(E) entities, each an
`AccessRoutes` call. The per-type **baseline computed once per (principal, type)
via the empty-entity probe** already collapses the common case to O(P·T); the
O(P·E) term is only the *exception* scan — entities reachable via a
role-relation or inheritance edge. So the reverse index is scoped as: **index
role-relation + inheritance edge sources so the exception scan visits only
entities that CAN carry an exception, not the whole entity space.**

## Cost — the reverse-index question ([[RR-K72ML0]], now due)

Confirm during planning:
1. Baseline stays O(P·T) — the empty-entity probe is per (principal, type), not
   per entity. Already true in `MapPrincipal`; whole-graph just loops it.
2. The exception scan is bounded by the set of entities that are a role-relation
   target or have an ancestor carrying a local grant — NOT every entity. Build a
   reverse index (edge-target → nothing-to-scan-otherwise) so a graph with
   thousands of baseline-only entities does O(edges), not O(P·E).
3. If the index is more than a slice's worth of work, whole-graph `map` MAY ship
   behind a documented "walks all entities" note and the index split to a
   follow-up — decide in planning, do not silently ship the O(P·E) loop as if it
   scaled.

## Acceptance criteria

### `can`
- [ ] `rela acl can <P> <verb> <E>` prints allow + the deciding route(s), or
  deny + "no grant", and sets a **non-zero exit code on deny** (scriptable).
- [ ] Read decided by the runtime read path (reuse `AccessRoutes` /
  `accessFor`); a missing entity errors distinctly (not a silent deny).
- [ ] `--format json` emits the versioned schema (a single-principal,
  single-entity `PrincipalAccess`-shaped object + a boolean).
- [ ] `principal_property` resolution + data-entry-transport caveat carried over
  in `--help`.

### Whole-graph `map`
- [ ] `rela acl map` (no `--principal`) lists every enumerated principal's
  effective access, per-type baseline + per-entity exceptions, each grant with
  all-routes provenance — same shape as `map --principal`, grouped by principal.
- [ ] `--verb` / `--type` filters apply across all principals.
- [ ] The `everyone` baseline is surfaced once (global), never repeated per
  principal.
- [ ] A **headline summary** (principal count, total grant-source count) reads
  the whole-graph posture at a glance.
- [ ] `--format json` reuses the versioned schema; deterministic ordering across
  principals AND within each principal.

### Correctness / cost
- [ ] Read-vs-runtime conformance holds for BOTH surfaces (a test asserts `can`
  ⟺ `PermitsRead`/`AuthorizeWrite`, and whole-graph `map` = the union of
  per-principal `MapPrincipal` runs).
- [ ] The exception scan visits only edge-reachable entities (a test with many
  baseline-only entities asserts the scan does not touch them) — OR the
  documented "walks all entities" fallback is in place with the index split to a
  named follow-up ticket.
- [ ] `just arch-lint`, `just plimsoll`, coverage floors.

## Tests
- [ ] `can` allow: a principal with a direct/group/edge grant → exit 0 + route.
- [ ] `can` deny: a principal with only the `everyone` baseline for a
  non-everyone verb → non-zero exit + "no grant".
- [ ] `can` missing entity → distinct error, not a deny.
- [ ] Whole-graph `map` = union of `map --principal` over the enumerated set
  (conformance).
- [ ] Whole-graph `map` with a baseline-only-heavy graph exercises the exception
  scan bound (cost guard).
- [ ] `--verb` / `--type` filters on whole-graph output.

## Explicitly deferred (later slices)
- Full hop-by-hop provenance chains (the `Source` restructure).
- Drift detection (UC8) — reuses this JSON as the snapshot format.
- Conformance assertions `rela acl verify` (UC4/UC5).
- Graph-aware heuristics folded into `rela acl audit` (UC7 full).
- Data-entry web view.

## Code review (cranky) — findings addressed

Reviewed after implementation; MCP tracker offline, so findings are recorded
here rather than as review-response entities. Read/update/delete false-negative
guarantee confirmed intact (both surfaces decide via the runtime read path).

- **SIGNIFICANT — one blank key aborted the whole-graph inventory.** `MapAll`
  ran `MapPrincipal` per candidate and propagated a blank key's
  `ErrUnstampedPrincipal`, so a single malformed assignment key or empty
  relation `From` turned the entire attestation into a hard error — unlike
  `who-can`, which skips blanks. FIXED: `MapPrincipal` returns an empty result
  for a blank/whitespace key and `MapAll` skips empty-principal rows. Regression
  test `TestMapAll_BlankKeyDoesNotAbort`.
- **SIGNIFICANT — no-policy `can` skipped the entity-existence gate.** With no
  `acl.yaml`, `rela acl can P v TYPO` short-circuited to ALLOW (exit 0) before
  `engine.Can` ran, so a typo'd id read as a green attestation. FIXED: the
  no-policy path (`runNoPolicy`) now checks `GetEntity` and errors "entity not
  found", matching the under-policy gate. Test
  `TestACLCan_NoPolicyMissingEntityErrors`.
- **INVESTIGATED, NOT A BUG — "create over-reports (globals-only)".** The review
  read `authz_write.go`'s `s.ID == ""` comment as "create is globals-only" and
  flagged edge-conferred create routes as a false ALLOW. But the PRODUCTION
  create path (`entitymanager.ApplyEntity`) authorizes with
  `EntitySubject{ID: e.ID}` — a concrete id — so the runtime takes the
  `s.ID != ""` branch and DOES fold local-edge routes into a create decision.
  Collapsing create to globals-only in the report would have introduced a false
  DENY (the worst class). Kept create computed with the concrete id; added
  `TestCreate_MatchesRuntime` + `TestCreate_EdgeConferredCreateReported` pinning
  `Can(create) == AuthorizeWrite(create)` in both directions, and a clarifying
  godoc on `grantingAttributions`.
- **MINOR — dedup comment vs code.** `MapAll`'s first-key-wins comment said
  "first non-empty result"; code stores the first result unconditionally.
  Corrected the comment to state the real invariant (access depends only on the
  effective User, so duplicate keys compute identical access; the who-can
  route-union asymmetry is called out) and added the blank-skip.
- **NIT — deny vs error both exit 1.** Documented in the command godoc: exit 0 =
  allow, 1 = deny OR engine error (fail-closed), missing entity is a distinct
  error, never a deny. A separate exit code for error-vs-deny deferred.
