---
id: RES-8TX9KF
type: research
title: 'Effective-access map: enumerate who can access what, with provenance and drift detection'
summary: 'ACL effective-access map: enumerate who-can-access-what with FULL-CHAIN, ALL-ROUTES provenance, filtering, and daily drift detection'
status: done
---

# Effective-access map / ACL audit tool

## Problem

rela's ACL system answers one question well: *"Can principal P do operation X to
entity E, right now?"* — evaluated per-request on the write path
(`AuthorizeWrite`) and per-type on the read path (`ReadQuery` /
`VisibleSearcher`). There is also a static **policy linter** (`rela acl audit`,
`internal/aclaudit`, Tiers A/B) that checks `acl.yaml` for structural smells
against the metamodel.

What is missing — and what this research scopes — is the **cross-product view**:
*"Who can access what across the whole graph, and why?"* An operator cannot
today verify that ACL is configured to **allow access where needed and deny it
where not** without tedious manual, per-principal, per-entity checks.
`internal/aclaudit` explicitly reserves this as future work:

> Tier C (graph-aware) and Tier D (reachability) are future work — `internal/aclaudit/aclaudit.go:20-21`

The goal is a tool that materializes the **effective-access relation**
(principal × entity × verb, with provenance) so it can be inspected, filtered,
gated in CI, and diffed over time.

## Grounding policy (used by all use-cases below)

An ISMS-style `acl.yaml` (from `docs/acl-overview.md`), extended with a
`security` group and an `incident` type so the least-privilege cases are real:

```yaml
user_entity_type: persoon
principal_property: email        # persoon.email holds the proxy header value
membership_relation: heeft_rol   # domain relation reused for group membership

roles:
  everyone:  { read: ["*"] }
  reader:    { read: [ticket, feature] }
  editor:    { read: [ticket, feature], create: [ticket], update: [ticket], delete: [ticket] }
  responder: { read: [incident], update: [incident] }
  security:  { read: [incident, ticket], create: [incident], update: [incident], delete: [incident] }

assignments:
  PERS-ALICE:      editor          # global grant to a resolved person
  ROLE-SECURITY:   security        # group grant; persons heeft_rol ROLE-SECURITY inherit
  jvloothuis@sourcehaven.nl: editor   # raw-UPN break-glass (not yet in graph)

role_relations:
  editor-of:   { confers: editor }    # an editor-of edge confers editor on its source
  responds-to: { confers: responder } # a responds-to edge confers responder

inherit_roles_through:
  - belongs-to                     # roles flow down containment (folder → contained docs)
```

## Use-cases (grounded)

Each has: **actor**, **trigger**, **scenario** against the policy above, **cares
because** (the cost of getting it wrong — the real motivation), the **command**,
**looks for** (the specific signal the actor scans the output for — pass vs.
fail), and **done =** the acceptance signal.

### UC1 — Onboarding verification *(inventory + spot-check)*

- **Actor:** admin who just granted access. **Trigger:** added `PERS-BOB` to `ROLE-SECURITY` (a `heeft_rol` edge).
- **Scenario:** Bob should now read/update `incident` via the group, and read `ticket`, and nothing else.
- **Cares because:** a group grant is coarse — it can carry *more verbs than intended* (delete, create) or reach types the admin never pictured. Adding the edge is trivial; knowing what it *actually* conferred is the hard part, and the admin owns the blast radius.
- **Command:** `rela acl map --principal PERS-BOB`
- **Looks for:** `delete = ·` on every type; a lone `✓` in a write column intent didn't call for is the red flag. Every `✓` should trace to `group ROLE-SECURITY → security`.
- **Done:** verb grid matches {read incident/ticket, update incident} and no more; a stray grant is visible on sight.

### UC2 — Offboarding / contractor review *(inventory, per-principal)*

- **Actor:** admin cutting off a leaving contractor. **Trigger:** account deprovisioning.
- **Scenario:** access accreted ad-hoc over months — `editor-of` edges, a group membership, maybe a raw-UPN break-glass line.
- **Cares because:** a single missed grant means *a departed account still reads live data* — an audit finding, or worse. Grants are scattered across assignments, group edges, and inherited folders; there is no one "revoke all," so the admin needs proof of the *full* set, not a plausible one.
- **Command:** `rela acl map --principal contractor@acme.example`
- **Looks for:** `everyone`-only output = fully cut off. Any Global / group / `editor-of` row names the exact edge still to delete.
- **Done:** output collapses to the `everyone` read baseline and nothing else — every ad-hoc grant accounted for and gone.

### UC3 — Sensitive-entity spot check *(inventory, per-entity + provenance)*

- **Actor:** security officer. **Trigger:** an `incident` entity holds breach details.
- **Scenario:** "Who can read INC-042, and by what path?" Access may arrive via the `security` group, a `responds-to` edge, or `belongs-to` inheritance.
- **Cares because:** this is a *confidentiality attestation* — the officer must say "these people, and only these, can see the breach." Inheritance makes readers invisible: one can be entitled by a folder three hops up that nobody remembers, so a bare name list isn't sign-off-able — the officer needs the *path* to judge whether each reader belongs.
- **Command:** `rela acl who-can read INC-042`
- **Looks for:** a `via belongs-to ancestor` reader is the one to scrutinize; a direct group / `responds-to` reader is expected and defensible.
- **Done:** every listed reader has a path the officer can name and defend; no "why is *this* account here?" line.

### UC4 — Regression after a config/graph edit *(conformance — needs expectations)*

- **Actor:** engineer refactoring `acl.yaml` or renaming a relation. **Trigger:** PR touching policy or containment edges.
- **Scenario:** renaming `belongs-to → contained-by` without updating `inherit_roles_through` silently drops all inheritance — responders lose access they *need*.
- **Cares because:** a lost-access bug is *invisible in a diff and silent at runtime* — the app doesn't error, users just quietly can't do their jobs, and it surfaces days later as a support ticket, not a stack trace. The engineer wants the PR to fail *now* if it broke access someone depends on.
- **Command (v2):** `rela acl verify` against `acl-expectations.yaml` (`expect: PERS-CAROL can update incident`).
- **Looks for:** "all N expectations hold" → merge; `FAIL: PERS-CAROL cannot update incident` → the rename broke inheritance.
- **Done:** non-zero exit naming each broken assertion — catches the under-provisioning direction anomaly heuristics cannot.

### UC5 — Least-privilege gate *(conformance — negative assertions)*

- **Actor:** compliance owner. **Trigger:** CI on every change.
- **Scenario:** invariant "nobody outside `security` may delete `incident`." A snapshot says *what is*; only an assertion says *what must hold*.
- **Cares because:** this is a control they've *signed their name to for an auditor* ("separation of duties on incident records"). It must hold across every future edit by people who've never heard of it, so eyeballing once isn't enough — it needs to be a standing gate that *stays* true and fails loudly the moment someone widens delete.
- **Command:** `rela acl who-can delete incident` (ad-hoc) / `expect: only ROLE-SECURITY members can delete incident` (gate).
- **Looks for:** only `ROLE-SECURITY` members listed — a single non-security principal = control broken, block the merge.
- **Done:** the gate fails CI if any principal outside `security` appears in the delete-`incident` set.

### UC6 — Blast-radius of a grant *(inventory diff / what-if)*

- **Actor:** admin weighing a new `editor-of` / `heeft_rol` edge. **Trigger:** someone asks for "edit access to the Q3 folder."
- **Scenario:** granting `editor-of` on a *folder* flows `editor` down every `belongs-to` descendant.
- **Cares because:** the request sounds narrow ("just that folder") but the grant is *transitive* — it can silently cover child folders and hundreds of entities the requester never mentioned. The admin wants to grant the least that satisfies the ask, and can't eyeball the descendant count from the edge alone.
- **Command:** `rela acl map --principal PERS-DAVE` before vs. after (diff two runs), or a future `--what-if edge=editor-of:PERS-DAVE→FOLDER-1`.
- **Looks for:** the *count of newly-reachable entities* in the diff — a number near the folder's size is fine; one an order of magnitude larger means the edge over-reaches.
- **Done:** the delta is scoped to what the request implied; an unexpectedly large descendant set is caught before the edge lands.

### UC7 — Config sanity *(heuristics — extend aclaudit)*

- **Actor:** any maintainer. **Trigger:** periodic / pre-PR.
- **Scenario:** assignment keyed to a `PERS-*` that no longer resolves; a role granting `update` but not the `read` it implies; a sensitive type left `everyone`-readable.
- **Cares because:** these are *latent misconfigurations that never throw an error* — the system keeps working while a dangling assignment quietly grants nothing (or an `everyone` rule quietly grants everything). The maintainer wants them surfaced before they become the incident, not after.
- **Command:** `rela acl audit` (extended with cheap graph-aware Tier-C checks).
- **Looks for:** 0 findings at the fail-on severity; each finding names a rule + subject + fix.
- **Done:** findings with severity + fix, same shape as today's aclaudit. Only catches *over*-provisioning + structural smells — not "access missing."

### UC8 — Access drift detection *(temporal diff — snapshot time-series)*

- **Actor:** security monitoring (scheduled). **Trigger:** daily `rela scheduler` run.
- **Scenario:** persist the map nightly; diff vs. yesterday. A bulk `heeft_rol` edit that grants 40 people `incident` access overnight; one person gaining one type is routine.
- **Cares because:** nobody is watching the graph edit-by-edit, so a *quiet, broad widening looks identical to normal churn* until it's abused. The value isn't any single day's map — it's noticing the *shape* of change: a spike means a bad migration, a leaked role edge, or an insider, and it must page someone the same night.
- **Command:** `rela acl map --format json > today.json` then `rela acl diff yesterday.json today.json --alert-swing 10`
- **Looks for:** deltas under threshold → silent; `+40 principals gained incident` → page.
- **Done:** per-principal gained/lost deltas; non-zero exit (or notify) past the swing. Reuses UC-A's JSON — which is *why* that output earns a stable, versioned schema.

**Coverage map:** UC1, UC2, UC3, UC6 need only the **inventory engine +
provenance** (v1, Option A). UC7 is **heuristics** (Option D, folded into
aclaudit). UC8 is **temporal diff** (Option C) reusing A's JSON. UC4/UC5 are
**conformance assertions** (Option B, v2) — the only mode that catches
under-provisioning and negative invariants.

## Provenance requirement (DECIDED — hard requirement)

"How was this permission acquired, via which route?" is answerable because the
resolver already tags every grant with a `Source` (the same attribution the
runtime used to enforce). The tool surfaces it, and two depth decisions are now
**fixed requirements**, not formatting preferences:

- **Full edge chain, not just a label.** Show every hop by name:
`PERS-DAVE → editor-of → FOLDER-Q3 → belongs-to ⤳ INC-042 → role editor`.
Rationale: "via inheritance" says access exists but not *which edge to cut*.
Offboarding (UC2) and revocation need the anchor entity + edge type by name, or
the operator is back to grepping the graph. The resolver already walks this
exact path in `computeForEntity`, so the chain is a byproduct, not extra work.
- **All routes when access is redundant, not the shortest.** When multiple paths grant the same permission (e.g. Dave reads INC-042 via *both* the `security` group *and* `belongs-to` folder inheritance), list **every** path. Rationale: cutting one path may leave access intact via another — showing only the "primary" route makes offboarding silently fail. The map must be truthful about redundancy so "fully revoked" means every path removed.

The six routes (the `Source` taxonomy, `internal/acl/source.go:8-15`):

| `Source` | Route | Example chain |
|----------|-------|---------------|
| `Global` | direct assignment | `PERS-ALICE → assigned → editor` |
| `Group` | via a member-of/heeft_rol group | `PERS-BOB → heeft_rol → ROLE-SECURITY → security` |
| `Local` | role-conferring edge to *this* entity | `PERS-CAROL → responds-to → INC-042 → responder` |
| `LocalViaGroup` | role-conferring edge from a group they're in | `ROLE-IR → editor-of → INC-042` (member inherits) |
| `LocalViaAncestor` | edge to a container, inherited down `belongs-to` | `PERS-DAVE → editor-of → FOLDER-Q3 ⤳ INC-042` |
| `LocalViaGroupAndAncestor` | both at once — group edge on an ancestor | the fully-compound case |

The compound routes (`LocalViaAncestor`, `LocalViaGroupAndAncestor`) are exactly
the grants manual review misses — access nobody granted *to this entity*
directly — so the route column is what makes them impossible to miss.
**Data-model implication:** the JSON schema (v1) must carry, per grant, an
ordered list of route objects, each an ordered list of `{entity, relation}` hops
ending in a role — not a single enum. This is baked in from day one because
UC8's snapshots and a future web view both consume it.

## Context

### The ACL model (as it exists)

- **Policy**: declarative `acl.yaml` at project root (never in the metamodel — arch-lint enforced). Lua does not participate on the read path. `internal/acl/policy.go:77-85`.
- **Principal**: `principal.Principal{User, Tool, RawUser}` — `internal/principal/principal.go:40-44`. Roles/groups are **not** stored on the principal; they are *derived* at request time from `acl.yaml` assignments + the graph.
- **Decision function**: `acl.ACL.AuthorizeWrite(ctx, WriteRequest) Decision` — `internal/acl/acl.go:61-63`. `Decision` carries `Attributions []RoleAttribution` — every allow/deny names the rule and role that fired.
- **Read path**: `Request.ReadQuery(ctx, entityType)` → `{AllowAll | DenyAll | Query}` (`internal/acl/readquery.go:27`); `search.VisibleSearcher.SearchVisible` with a per-type `TypeScope` map (`internal/search/types.go:105`), fail-closed.
- **Resolver / provenance**: `computeGlobals` (`internal/acl/resolver.go:15`) walks `member-of` closure; `computeForEntity` (`resolver.go:118`) crosses the member set × the entity's ancestor chain (`inherit_roles_through`) against `role_relations`. Provenance is the **`Source` taxonomy**: `Global`, `Group`, `Local`, `LocalViaGroup`, `LocalViaAncestor`, `LocalViaGroupAndAncestor` — `internal/acl/source.go:8-15`, documented in `docs-project/entities/concepts/CON-authorization.md:53-63`.

### The principal universe (net-new enumeration)

No existing code enumerates all principals — the resolver is always pull-based
from one known `principal.User`. To build the map, the union is:

```
(all entities of user_entity_type)         # resolvable human principals; principal_property is unique
∪ (all keys in assignments)                # direct + group assignment keys
∪ (all sources of membership_relation edges)  # anything reachable in a member-of closure
∪ (all sources of role_relations edges)    # holders of role-conferring edges
```

Plus the built-in **`everyone`** role (`EveryoneRole`, `policy.go:33`), appended
to every principal — the map must represent this as a distinct pseudo-principal,
not silently omit it. The reusable enumeration primitive is
`store.EntityReader.ListEntities`; the reusable resolution primitive is
`Declarative.ForPrincipal(p)` → `computeForEntity` per entity.

### Constraints

- **Scope caveat (important):** `principal_property` resolution is wired into **data-entry transport only** (`internal/acl/declarative.go:112-126`). CLI/MCP/scheduler/desktop authorize against the *raw* principal. An entity-keyed map reflects data-entry-transport access; the tool must document this rather than imply universal coverage.
- **Read-path purity:** no user-supplied Lua on the read path (CLAUDE.md). The map is built from declarative policy + graph reachability only — consistent with the existing evaluation model.
- **Consumer-side interfaces:** the map engine should take narrow interfaces (`ListEntities`, `ForPrincipal`, policy accessors), not a service locator — per CLAUDE.md rules.
- **CLI surface:** extend the existing `rela acl` command family (already hosts `audit`) — `internal/cli/acl.go`.
- **Scale:** real projects have thousands of entities; a full principal×entity matrix is O(P·E). Default output must **aggregate by entity type + list only per-entity exceptions** (where graph/inheritance makes one entity differ from its type baseline).
- **Future web surface:** engine emits stable structured (JSON) output so a data-entry web view can consume it later without re-deriving.

## Options

### Option A — Inventory engine + query/filter CLI (recommended v1)

A new package (e.g. `internal/aclmap`) that enumerates the principal universe,
and for each principal computes effective access, aggregated by entity type with
per-entity exceptions, every row carrying its full-chain, all-routes `Source`
provenance. Exposed as:

```
rela acl map [--principal P] [--type T] [--entity E]
             [--verb read|create|update|delete]
             [--via global|group|relation|inheritance]
             [--format text|json]
rela acl can <principal> <verb> <entity|type>   # yes/no + reason, exit code (UC1)
rela acl who-can <verb> <type|entity>           # list principals + provenance (UC3/UC5)
```

- **Covers:** UC1, UC2, UC3, UC6 (via two runs), UC7 (partially, by surfacing everyone-readable etc.).
- **Pros:** reuses existing resolver primitives; no new policy syntax; is the shared foundation every other mode builds on; the exact engine a web view needs; provenance-first (audit, not just a grid).
- **Cons:** O(P·E) worst case — mitigated by type-aggregation + exceptions; entity-keyed map has the data-entry-transport caveat.
- **Effort:** M (enumeration + aggregation + full-chain provenance formatting + CLI wiring + conformance tests).

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

- **Covers:** UC8. Reuses Option A's JSON as the snapshot format — so that output earns a stable, diffable shape.
- **Pros:** turns the snapshot from a *spec you maintain* (its weakness for UC4) into an *automatic time-series* (its strength); composes with the scheduler + append-only audit log; no expectations file.
- **Cons:** needs a snapshot store + diff/magnitude thresholds; noisy on legitimately large changes (needs a swing threshold).
- **Effort:** M on top of A (snapshot persistence + diff + threshold alerting). **v2/v3.**

### Option D — Extend aclaudit into Tier C/D directly

Rather than a new package, grow `internal/aclaudit` with graph-aware (Tier C)
and reachability (Tier D) checks, emitting the same `Finding` shape.

- **Covers:** UC7 fully, parts of UC4/UC5 as heuristics (over-provisioning only).
- **Pros:** reuses the finding/severity/CLI machinery; matches the reserved-tier design intent; heuristics need no expectations file.
- **Cons:** aclaudit's `Finding` model is lint-shaped (issues), not inventory-shaped (a map) — forcing the full map through it is awkward. Heuristics can only ever catch *over*-provisioning, never "access missing where needed."
- **Effort:** S per check; but does not deliver the map/inventory itself.

## Recommendation

**Ship Option A first** (inventory engine + `map`/`can`/`who-can`,
provenance-first, type-aggregated). It directly answers 5 of the 8 use-cases, is
the reusable engine every other mode and the future web view consume, and
requires no new policy syntax. Fold **Option D's cheap graph-aware heuristics**
into `rela acl audit` opportunistically (they share the metamodel already).

Then layer, in order of value:
- **Option C (drift, UC8)** — high operational value, reuses A's JSON as the snapshot format, composes with the scheduler. This is what makes A's output earn a stable schema.
- **Option B (assertions, UC4/UC5)** — the only mechanism that expresses "access **where needed**" (under-provisioning) and negative least-privilege invariants. Deferred so its syntax is shaped against real engine output.

**Tradeoffs accepted:** (1) the entity-keyed map reflects data-entry-transport
access due to the `principal_property` resolution caveat — documented, not
hidden; (2) O(P·E) cost mitigated by type-aggregation + exceptions, not
eliminated; (3) `everyone` is surfaced as an explicit pseudo-principal so
anonymous/unbounded access is never silently dropped; (4) **provenance is
full-chain + all-routes** — the JSON schema carries an ordered list of route
objects per grant, each an ordered list of `{entity, relation}` hops, so
revocation is actionable and redundant access is never hidden.
