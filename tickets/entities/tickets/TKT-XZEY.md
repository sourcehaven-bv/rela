---
id: TKT-XZEY
type: ticket
title: 'ACL relation permissions: design record + umbrella (split into TKT-K2VN9D / TKT-VR61XC / TKT-8HDPQW)'
kind: enhancement
priority: high
effort: m
status: backlog
---

## SPLIT INTO INCREMENTS (2026-08-19)

**This ticket is now the DESIGN RECORD, not the implementation ticket.** After
design review it was split so the work lands reviewably and incrementally. Do
not implement from this ticket — implement from the increments, which each carry
the constraints they need.

| Ticket | Scope | Effort | Depends on |
|---|---|---|---|
| **TKT-K2VN9D** | Increment 1 — `relation_grants:` block: config, validation, `authorizeRelationWrite` seam | m | — |
| **TKT-VR61XC** | Increment 2 — `rela acl can` relation verification | s | K2VN9D |
| **TKT-8HDPQW** | Increment 3 — cascade delete under `Store.Tx` (B1) | m | K2VN9D |
| **TKT-JW03LN** | Incoming-side renumber authz + `OpRename` pin (DR-5, DR-25) | s | K2VN9D |
| **TKT-7QM4RB** | Carry `allow_acl_bypass` through declarative `create_relation:` | m | — |
| **TKT-M3W8PK** | Automation relation writes as triggering principal (B3) + autocascade fatal-error contract | l | 7QM4RB |

Increment 1 is independently shippable and independently useful: it fixes the
motivating incident, which was a **Lua** `create_relation` — already routed
through `EntityManager` and therefore already covered by
`authorizeRelationWrite`.

Increment 2 is **not optional polish**: this ticket names TWO root causes, and
increment 1 alone fixes only the first while making the second (invisibility)
marginally worse by adding an allow source no tool can see.

What remains below is the design rationale, the security review outcome, the
coverage map, and the full design-review findings (DR-1..DR-26) — the reasoning
the increments depend on and must not re-litigate.

---

## Status

**Resolved (2026-08-19): option (a).** The central design question below — "(a)
wire-level relation verbs vs (b) full Lua delegation" — is answered in favour of
**(a)**, with a concrete shape. Scope is now per-relation create/update/delete
permissions declared in `acl.yaml`, keyed by relation type. `read:` is
deliberately excluded (see below). The Lua write-veto prerequisite is **no
longer blocking**: option (a) needs no Lua.

Driven by a real deployment incident — see "Motivating bug" below.

## Motivating bug

Relation writes resolve the verb grant against the **source entity's type**
(`internal/acl/authz_write.go:60-63`):

```go
attrs := r.Globals(ctx).Attributions
return r.decideFromAttrs(attrs, op, s.FromType,
    "no role grants %s on relations from type %q")
```

So the verb tracks the **relation's** lifecycle while the type it is checked
against is the **source entity's**. `create: [terugkerend]` reads as "may create
terugkerend entities" but is really doing double duty as "may add edges from
terugkerend". There is no way to grant edge-creation without also granting
entity-creation — strictly more authority than intended.

In the incident, a scheduled Lua task did:

```lua
local taak = rela.create_entity("taak", props, body)   -- OK
rela.create_relation(rec.id, "spawnt", taak.id)        -- DENIED
```

The policy granted `update: [terugkerend]`. `rela acl map`, `acl can` and `acl
audit` all reported healthy — none of them can express a relation-shaped
question. The failure was **partial, not atomic**: `create_entity` had already
committed, and the script's dedup guard keyed off the very edge that was never
written, so every tick created another duplicate until the process was
OOM-killed.

Two properties turned a permissions typo into an outage: **no verb that
expresses "may add an edge"**, and **silent under every static check**.

## Design

Top-level `relations:` block in `acl.yaml`, keyed by relation type:

```yaml
relations:
  spawnt:
    create: create-spawnt
    update: edit-spawnt
    delete: remove-spawnt
```

with a shorthand gating all three on one permission:

```yaml
relations:
  spawnt:
    permission: manage-spawnt      # create + update + delete
```

Values are ACL **permission names** — the existing capability-noun vocabulary
checked by `Request.holdsPermission` → `grantsPermission`
(`internal/acl/resolver.go:267`), granted by `RoleDef.Permissions`.

### Why acl.yaml and not schema.yaml

The gate declaration belongs in `acl.yaml`, **not** on `metamodel.RelationDef`.
The rule: **only a layer that already depends on both schema and ACL may
reference both.** `data-entry.yaml` qualifies (composition layer — its four
`Permission` fields are fine). `schema.yaml` does not: it is the domain model
and must stay readable and enforceable with no ACL present.

Keying by relation *name* needs no from/to duplication — the metamodel already
constrains both endpoints declaratively, and `acl.Policy` already keys maps by
graph-vocabulary names it does not own (`RoleRelations`, `MembershipRelation`,
`InheritRolesThrough`).

`TransitionDef.Guard` in `schema.yaml` is the counter-example, and is considered
a **bug** — its own godoc admits it is "enforced only on served paths; inert on
direct CLI writes", i.e. a security gate that silently does not apply depending
on entry point. **Follow-up ticket to be filed** to fix `guard:` along these
lines once this lands.

### Why read is excluded

Relation reads do not go through `authorizeRelationWrite` at all. They go
through `visibility.PolicyReader.FilterRelations`
(`internal/visibility/policyreader.go:128`), where visibility is **derived, not
granted**: a relation survives iff BOTH endpoints are visible (FROM ∧ TO),
fail-closed.

That is not an oversight. A relation's existence reveals that both endpoints
exist, so an independent `read: spawnt` grant would be an **entity-existence
oracle** — precisely what the row-level uniform-404 rule in CLAUDE.md protects.
A per-relation read permission could therefore only ever NARROW (both endpoints
visible AND permission held), never widen. That is a different mechanism
(visibility decorator), a different polarity (additional-only vs the
sufficient-grant CUD semantics), and it touches `ReadQuery`'s SQL pushdown.

Keeping `read:` out of the block is deliberate: putting all four verbs in one
block would invite the assumption that `read:` behaves like the others.

## Open design questions

**Note (2026-08-19, post security-review):** questions 1 and 2 below are
ANSWERED — see "Security review outcome". The word "sufficient" in the original
framing was the defect; do not reintroduce it.

## Security review outcome (2026-08-19) — READ BEFORE IMPLEMENTING

A security trace of the proposed design against the existing hardening found
that the original framing — *"holding the named permission is SUFFICIENT to
authorize the relation write"* — **breaks two security properties**. Both come
from the same mistake: making the new check a **short-circuit allow** instead
of an **additional allow source evaluated after the existing clamps**.

### Correct semantics (supersedes "sufficient")

> A `relations:` permission is an **alternative satisfier of the source-type
> verb grant (gate B) only.** It is NEVER "sufficient to authorize the write".

Today the decision is a conjunction (`authz_write.go:46-66`):

```
allow = (delegate-X satisfied OR not configured)   # gate A, :48-58
      AND ceiling permits verb on FromType          # inside decideFromAttrs, :87
      AND some role grants verb on FromType         # :99-107
```

The new block may only widen the **third** conjunct. The first two must remain
unconditional.

### Break 1 — delegate-X / RR-7O6Q self-promotion (critical)

`relations: {member-of: {create: some-perm}}` alongside
`role_relations: {member-of: {requires_permission: delegate-membership}}` would
let a principal holding `some-perm` but NOT `delegate-membership` write
`alice --member-of--> admins` and self-promote to any role assigned to a group.
That is a verbatim reinstatement of the attack RR-7O6Q exists to prevent
(`policy.go:394-421`), which `docs/acl-security.md:9-38` calls mandatory to
gate and `aclaudit` flags at severity High (`tier_a.go:81-101`).

**Rule:** gate A (`authz_write.go:48-58`) stays structurally FIRST and
unconditional. The `relations:` check must never be placed at or above it, nor
OR-ed around the function body.

**Belt-and-braces:** `Policy.Validate` HARD ERROR when a relation type appears
in both `relations:` and a `role_relations:` entry with a non-empty
`requires_permission` — two rules contradicting each other about one relation
type, with escalation as the resolution. Precedent: the `scope_grants`
deny-spelling rejection (`ceiling.go:318-326`).

### Break 2 — client ceiling would widen (critical, unconditional)

More severe because it fires for EVERY deployment using client attenuation, not
only those using groups.

`filterTypes` (`ceilingcompile.go:277-301`) deliberately **preserves** a role's
`"*"` wildcard under a pure denial, because a plain allowlist cannot spell "all
except X" (`:286-292`). So under `deny_write: ["*"]` — the documented read-only
client one-liner (`ceiling.go:129-134`) — the clamped `RoleDef` returned by
`roleFor` STILL CONTAINS `"*"`. The actual denial is enforced by
`r.ceiling.permitsVerb` at `authz_write.go:87`, inside `decideFromAttrs`.

**Therefore: a `relations:` check placed BEFORE or INSTEAD OF `decideFromAttrs`
escapes the ceiling entirely** — a read-only PAT could create, update and delete
relations. That violates `ceiling.go:13-25` ("A ceiling NEVER grants") and the
CLAUDE.md rule "A ceiling only ever NARROWS".

**Rules:**
1. `r.ceiling.permitsVerb(op, s.FromType)` must still be evaluated and must
   still be able to DENY even when the `relations:` permission is held.
2. The permission check MUST route through `r.holdsPermission` /
   `r.grantsPermission` — never `slices.Contains(policy.Roles[...].Permissions)`
   or any fresh traversal. `grantsPermission` (`resolver.go:272-289`) applies
   `permitsPermission` at `:275` and `roleFor` at `:280`, so routing through it
   inherits BOTH ceiling axes for free. `filterPermissions`
   (`ceilingcompile.go:302-320`) has the same wildcard-preservation behaviour,
   so a fresh lookup escapes the permission axis too.
3. **Preferred structure:** leave `authorizeRelationWrite` returning through
   `decideFromAttrs`, and pass the `relations:` grant in as an additional allow
   source checked AFTER the ceiling test at `:87`. This reuses the existing
   ceiling placement rather than re-deriving it.

### Not a new hazard (keep it that way)

Relation writes read `r.Globals(ctx).Attributions` (`authz_write.go:63`), NOT
`computeForEntity` — so relation writes fold in no local roles (contrast
`authorizeEntityWrite` at `:39`). Sourcing the new permission from local roles
would let a locally-conferred role authorize edge creation: the delegate-X
inversion in a different dress. Keep the new check on globals only.

### Guard-test blind spot (must be closed by this ticket)

`ceilingguard_test.go` scans every non-exempt file in `internal/acl` for the
regex `policy\.Roles\[` (`:34`) — the pattern that reaches a `RoleDef` without
passing `roleFor`. It is an EXEMPTION list, so a new file fails closed
(`:16-21`), and it has an anti-vacuity floor of 5 scanned files (`:120-123`).

**Blind spot:** the regex matches only `.Roles[`. A new `policy.Relations[...]`
read that decides an allow is INVISIBLE to it, yet escapes the ceiling just as
thoroughly. This ticket must add a positive test: *a `deny_write: ["*"]`
ceiling still denies a relation write for a principal holding the `relations:`
permission.* Prose in a godoc will not catch the regression.

A new implementation file (e.g. `internal/acl/authz_relations.go`) must NOT be
added to `exemptFiles` — every current exemption is "no principal in scope",
and a relation-authz file has a principal in scope by definition. Expressing
the check as `r.grantsPermission(attrs, perm)` satisfies the guard for free.

### Validation (Policy.Validate — no metamodel needed)

1. Blank relation-type key → hard error (parallel to the `role_relations` check
   at `:665-669`; a blank key would grant the permission path on EVERY relation
   type).
2. Blank permission name under any verb → hard error. Fails closed, but
   silently-inert config is the failure mode operators least notice — the
   codebase consistently rejects it (`policy.go:544-549`, `ceiling.go:246-249`).
3. Relation type in both `relations:` and gated `role_relations:` → hard error
   (Break 1 above).
4. Unknown verb keys under a relation entry (anything but create/update/delete)
   → reject. `read:` there is meaningless — relation reads gate through the
   row-level visibility path.
5. Add the new key to `knownPolicyKeys` (`policy.go:429-443`) — otherwise
   LoadPolicy warn-and-ignores it (`:465-471`). **`TestKnownPolicyKeysMatchStruct`
   (`policy_parity_test.go`) enforces this**, so the test fails until it is done.
   Add a `normalize*` pass trimming keys and permission names (RR-IK355A
   precedent, `policy.go:544-563`).

**Version-skew footgun to document:** unknown top-level keys are
warn-and-continue, not fatal (`policy.go:465-471`). On an older binary a
`relations:` block is silently ignored — a policy that LOOKS like it grants
edge-creation but does not. Safe direction (fails closed), but an operator who
believes it is active and relaxes something else is not safe. Must be called out
in the docs.

### Metamodel-dependent checks → `aclaudit` (advisory) or `ValidateAgainstMetamodel` (hard)

Note `Policy.ValidateAgainstMetamodel` (`policy.go:772`) DOES exist and is run
at the wiring site (`appbuild.go:842`) as a hard boot error, via the narrow
`MetamodelView` interface. Adding `HasRelationType` to that view is a small,
well-precedented change — an undeclared relation type in `relations:` could
therefore be a BOOT ERROR rather than only an advisory audit finding. Decide
which; boot-error is the stronger option and matches the existing treatment of
an undeclared `user_entity_type`.

Advisory (`aclaudit`, needs no new interface): a named permission no role
grants (dead config — same shape as the existing `tier_a.go:160` check).

### Naming (decide before writing code)

`relations:` is ALREADY an `acl.yaml` key — `RoleDef.Relations`
(`policy.go:298`), nested under a role, keyed by ENTITY type, consumed by
`internal/affordances`. A top-level `relations:` keyed by RELATION type would
be the same word at a different nesting level with a different key space and
different semantics. It is also already user-facing documented as "a separate
grant vocabulary (`relations:` on a role)" — `docs/acl-overview.md:456`.

**Recommendation: rename the new block** (`relation_writes:` or
`relation_grants:`) rather than overload the word.

## Original open design questions (1 and 2 now answered above)

## Coverage map (2026-08-19) — where a new check does and does NOT apply

A full trace of every relation-write path. **`acl.RelationSubject` has exactly 5
non-test construction sites**; everything else either routes through them or
bypasses the gate entirely.

### Gated today (a new check inside `authorizeRelationWrite` covers these for free)

| Path | Site | Via |
|---|---|---|
| Lua `rela.create_relation` / `delete_relation` | `lua/runtime.go:2226,2248` | `EntityManager.*` (this is the incident path) |
| MCP relation tools | `mcp/tools_relation.go:86,120` | `EntityManager.*` |
| CLI `link` / `unlink` | `cli/link.go:18`, `unlink.go:20` | `EntityManager.*` |
| Sync pull/splice | `cli/sync/splice.go:85`, `pull.go:302` | `ApplyRelation` / `DeleteRelation` |
| Dataentry relation handlers | `write_handler.go:637,711,765`; `relations_modern.go:333,376,415` | `manager.*` |
| Relation-history restore (CLI + SPA) | `cli/relation_history.go:201,205`; `relation_history_handler.go:368,370` | `EntityManager.*` |
| Git conflict-resolve | `write_handler.go:928` → `translateRelationWrite` | direct `AuthorizeWrite` |

### NOT gated — decide explicitly per row, do not let these be oversights

| # | Path | Site | Status |
|---|---|---|---|
| B1 | **Cascade delete** | `manager.go:911` (`Store.DeleteEntity(cascade)`); only ACL call is the ENTITY delete at `manager.go:855-861` | **IN SCOPE — decided 2026-08-19.** Authorize every incident relation up-front; any denial **fails the whole entity delete**. See "Cascade delete decision" below. |
| B2 | **Entity rename** re-key | `rename.go:36` → `fsstore/entity.go:430-460` | Defensible (relation SET unchanged; only endpoint ids move, covered by the entity `rename` grant). Answers open question 5: **no caller ever pairs `OpRename` with a `RelationSubject`**, so `grantsVerb`'s `OpRename→Update` routing is unreachable for relations. Make it an explicit decision, not an oversight. |
| B3 | **Automation / autocascade `create_relation`** | `cascadehost.go:140-155` — `WriteRelation` calls `Store.CreateRelation` **directly, no ACL call at all**; audit only | **SPLIT OUT at design review (DR-2) → TKT-M3W8PK**, blocked on **TKT-7QM4RB**. Decision unchanged (gate as triggering principal); the *cost* was 3-5× the estimate — it needs an `autocascade.Outcome` fatal-error contract change and cross-package bypass plumbing. |
| B4 | Lua `rela.bypass_acl` elevated handle | `lua/runtime.go:1988,2000` | Deliberate + audited (`OpACLBypass`, `manager.go:334-338`), gated by `allow_acl_bypass`. Fine. |
| B5 | Managed-order renumber | `manager_order.go:197` direct `Store.UpdateRelation` | **NOT "fine" — see DR-5.** True for the OUTGOING side; the INCOMING query (`:222`) spans siblings with different FROM entities of different types, writing relations the principal never authorized. Must authorize the incoming plan entries. |
| B6 | Dataentry soft-condition fallback | `relations_modern.go:393,434` direct `store.*` | **Conditionally safe and a constraint on placement**: only reached AFTER the manager call already returned a soft metamodel-allowlist error, i.e. the ACL already allowed it. Safe **iff** the new check lives inside `authorizeRelationWrite` (manager fails first, fallback unreachable). If enforced anywhere LATER than the manager's authz, these two lines bypass it. |
| B8 | Importer | `importer/importer.go:449` direct `store.CreateRelation`, no `acl` import | Trust boundary is operator shell, like `db migrate`. Out of scope, but state it. |
| B9 | Docs seeding | `docs/module.go:118`, `seed.go:43` | Build/seed-time tooling. Out of scope. |

### Cascade delete decision (B1) — deny the whole delete

A cascade delete authorizes **every** incident relation; a single denial fails
the **entire** entity delete. Deleting an entity must not be a back door to
destroying edge types you cannot delete directly.

> **CORRECTED 2026-08-19 by design review — see DR-1.** The original rationale
> below ("pure pre-flight ⇒ no race") is **WRONG** and must not be relied on.
> The pre-flight authorizes a snapshot collected OUTSIDE any lock, and both
> stores independently RE-DERIVE the relation set inside their lock/tx
> (`fsstore/entity.go:308-313`, `pgstore/entity.go:371-382`). Concurrent writers
> can add incident relations in the window, and those are deleted unauthorized.
> **B1 must run collect+authorize+delete under `store.Store.Tx`.** Struck text
> retained only to show what was believed and why it failed.

~~This is cheaper and safer than it first appears:~~

- ~~**Pure pre-flight.** `incoming`/`outgoing` are already collected at
  `manager.go:862-869`, *before* the first mutation.~~ **WRONG — the store
  re-derives its own set inside the lock. See DR-1.**
- ~~**Postgres is transactional regardless.**~~ **Non-sequitur — the tx does
  not span the authorization; READ COMMITTED sees rows committed after it began.**
- **fsstore/memstore have a write mutex, not a transaction**
  (`fsstore/tx.go:60-64`). Still true; `Store.Tx` is the sanctioned seam that
  makes the check and the delete mutually exclusive (DEC-8UIL0).
- **Document the consequence:** an entity delete can now fail on a *relation*
  grant. Intended least-privilege semantics, but a behaviour change operators
  must be told about.

### Automation decision (B3) — gate as the triggering principal

`cascadeHost.WriteRelation` must authorize the relation write with the
**triggering principal's** authority — the same principal whose write started
the cascade. No implicit elevation.

**Why: this restores the codebase's own stated pattern.** `allow_acl_bypass`
(`metamodel/types.go:646-656`) is operator-only, declared in `schema.yaml`, and
since TKT-Y3JVFK is an ENUM (`read` / `write` / `read+write`) so the operator
names exactly which elevation is unlocked; legacy `true` is refused at parse
time. Elevation is closure-scoped, **still audited, real principal preserved**.
So the invariant is: *ACL applies by default; bypass is declared, narrow and
audited.*

B3 breaks that on both counts — no ACL call AND no declaration. The asymmetry is
explicit in the same struct: `AllowACLBypass` is documented "**Ignored for
non-Lua actions**", so a Lua action must DECLARE bypass to skip the gate while
the declarative `create_relation:` action skips it unconditionally. Backwards.

**`create_entity` is not a counter-example.** `cascadeHost.CreateEntity`
(`:46`) routes through `createCore`; `WriteEntity` (`:91`) re-runs metamodel
validation, `checkUniqueProperties` and `Transitions.EnforceCreate`, with the
comment "the create path must not be the weaker one" (BUG-KIMZRK). Only
`WriteRelation` does neither.

**Failure semantics — the real work.** Unlike B1 (pre-flight over a
pre-collected slice), B3 fires INSIDE the cascade runner, mid-run, where earlier
steps may already have committed. This is the same non-atomicity that produced
the motivating incident. A denial must **abort the cascade and surface the
error**, never skip-and-continue — a silently-skipped edge is precisely the
dedup-guard failure that caused the outage.

**Migration break (document loudly):** an automation that works today can start
failing when the triggering user lacks the relation grant. That is the correct
semantics, but it IS a behaviour change. An operator who genuinely needs
elevation declares `allow_acl_bypass` on the action — which requires extending
that field's honouring to non-Lua actions, currently "ignored" there.

Effort moves to **l** with B3 included.

### `FromType` is empty in 4 of 5 sites

Sites 1–3 (`manager.go:1090,1174,1244`) and 5 (`affordances.go:66`) obtain
`FromType` best-effort and leave it **empty** when the source entity is missing
or unreadable (deliberate — BUG-K6FEVB requires authz BEFORE the peer-existence
lookup). Only `ApplyRelation` (`apply.go:243`) always populates it, because
`requireEndpoint` hard-fails first.

Today an empty `FromType` fails closed only because no role lists `""` — **except
a role holding `"*"`, which matches an empty target** (`grantsVerb`,
`policy.go:317-320`).

> **CORRECTED 2026-08-19 — see DR-4.** The earlier claim that a `s.Type`-keyed
> check is "a robustness improvement, unaffected by this class of problem" is
> **WRONG**. It is true of the new check in isolation and false of the
> CONJUNCTION: once the relation permission is an alternative satisfier, an
> unresolvable source no longer has to satisfy anything, so
> "source unresolvable ⇒ deny" silently becomes "source unresolvable ⇒ allow".
> The new allow source MUST be gated on `s.FromType != ""`.

### dataentry `lint_test` invariant

`TestNoStrayWriteRequestConstruction` (`dataentry/lint_test.go:28`) fails if any
non-test file in `internal/dataentry` **other than `affordances.go`** contains
the literal `acl.WriteRequest{Op:`. A new dataentry relation path must call
`translateRelationWrite` or go through the manager. Note the guard is a
substring grep: defeated by whitespace, field reordering, or a subdirectory
package — and it does not constrain `internal/entitymanager` at all.

### How this relates to `RelationGrant` (must be answered, not left open)

The two tracks are now fully mapped and they are **not equivalent**:

| | keyed by | enforced where | default |
|---|---|---|---|
| `acl.RelationGrant` → `affordances.RelationVerdicts` | relation type (under an entity type, under a role) | **`internal/dataentry` HTTP layer only** (`affordances.go:560,605,680`, called from `write_handler.go:417,626,630,683,758`) | **permissive** — unlisted type returns nil (`affordances.go:568`) |
| `authorizeRelationWrite` | source ENTITY type (+ delegate-X per relation type) | every write path | deny |

So `RelationGrant` is genuinely enforced, but **only on the SPA path** — not
Lua, MCP, CLI, sync, or automations. That is why an operator can write a
`RelationGrant`, see it honoured in the UI, and still be denied (or allowed!)
elsewhere.

Placing the new permission inside `authorizeRelationWrite` gets universal
coverage — but the ticket MUST state how it relates to `RelationGrant.Create`/
`Remove`, or operators face two overlapping YAML surfaces meaning
almost-but-not-quite the same thing.

**Test-coverage note:** `affordances_contract_test.go` pins an
"`_actions[v]==false` ⇒ 403 on the write" contract for *entity* verbs, but
**there is no equivalent contract test pinning relation verdicts to relation
writes** — which is precisely how the two tracks were able to diverge. Adding
one is in scope.

## Open design questions

1. **Polarity per verb — sufficient or additional?**
   - `create` **must be sufficient** (holding `create-spawnt` works with no
`create: [terugkerend]`), or the motivating bug is not fixed.
   - `delete` sufficient is more dangerous — it lets a principal remove edges
from entities it otherwise cannot touch. Needs a deliberate decision, not
symmetry-by-default.
2. **Delegate-X interaction (security-critical).** `authorizeRelationWrite`
checks `RoleRelationDef.RequiresPermission` FIRST, then the type gate. A
*sufficient* `relations:` permission must NOT become a bypass around
delegate-membership, or `relations: {member-of: {delete: foo}}` strips a role
without holding `delegate-membership` — undoing the RR-7O6Q self-promotion
hardening. Proposed rule: if a relation type appears in `role_relations:`, the
delegate gate always applies for **every** verb, and a `relations:` permission
can satisfy only the type-gate half.
3. **Shorthand + explicit coexistence.** Simplest rule: mutually exclusive,
rejected by `Policy.Validate` if both appear. A per-verb override on top of a
bundle is a second union semantic in a block already introducing one.
4. **Naming collision.** `RoleDef.Relations` (affordances, keyed by entity
type under a role) and this new top-level `relations:` (keyed by relation type)
would both be called "relations" in the same file. Distinguishable, but the
naming needs deciding before it ships.
5. **`OpRename` routing.** `grantsVerb` routes `OpRename` through `Update`.
Confirm that is right for relations, given the entitymanager captures a `rename`
version per incident relation.

## Notes on existing surfaces

- **`Op` plumbing already exists.** The four entitymanager call sites already
pass relation-lifecycle-accurate Ops — `CreateRelation`→`OpCreate`
(manager.go:1090), `UpdateRelation`→`OpUpdate` (1175),
`DeleteRelation`→`OpDelete` (1245), `ApplyRelation`→ by existence
(apply.go:238). Only the *target type* is wrong. No new Op-layer plumbing.
- **`update` is real.** Relations carry `Properties` and `Content`
(`RelationDef`), so `UpdateRelation` genuinely mutates edge metadata.
- **`RelationGrant` already exists but is NOT the write gate.**
`acl.RelationGrant` (`policy.go:353`) has per-relation `create`/`remove` with
`when:` predicates — but it is consumed only by `internal/affordances`
(`resolver.go:243`), never by `authorizeRelationWrite`. It is also closed-world
opt-in per type, where `decideFromAttrs` is an allowlist union. Two parallel
relation-permission systems with different semantics; an operator can write a
`RelationGrant`, see it honoured in the SPA, and still be denied at runtime on
the Lua path. This ticket must state how the two relate.
- **Visibility fix is a side effect.** Because the key is a relation *name*,
`aclaudit` can cross-check it (`MetamodelReader.GetRelation` already exists;
`checkUndeclaredRelations` is the check to extend), so a typo is caught by `rela
acl audit` instead of at runtime. `Policy.Validate` cannot — it has no
metamodel.

## Out of scope

- `read:` on relations (see rationale above).
- Fixing `TransitionDef.Guard` — **follow-up ticket, to be filed after this
lands.**
- The partial-write / non-atomic multi-step Lua script problem from the
incident (a denied second write leaves the graph in a state the script's own
idempotency check misreads). Distinct bug, deserves its own ticket.
- Per-property / per-value write gates (dissolved in Reframe #2 below).
- Read-side filtering and property redaction.

---

# Design review findings (2026-08-19)

Two independent reviews (adversarial-security, and design/completeness). They
converged on the same three criticals. **All three invalidate load-bearing
claims previously written in this ticket** — those claims are corrected in place
below and struck from the sections above where they appeared.

## CRITICAL — blocking

### DR-1 (F1) — B1's "pure pre-flight ⇒ no race" is WRONG. TOCTOU in cascade delete.

The claim "authorization happens with nothing yet written, so no rollback is
needed" is true about *mutation ordering* and false as a *safety* argument.
The pre-flight authorizes a snapshot collected **outside any lock**
(`ListRelations`, `core.go:240-254`, called at `manager.go:861-869`). Both
stores then **independently re-derive** the set inside their lock/tx and delete
*that* set:

- fsstore rebuilds from the live index under `s.mu.Lock()`
  (`fsstore/entity.go:308-313`) — VERIFIED by direct read.
- pgstore re-reads inside the tx (`pgstore/entity.go:371-376`) and deletes with
  an unqualified `DELETE FROM relations WHERE from_id=$1 OR to_id=$1` (`:382`).
  The tx begins at `:357`, long AFTER the manager's collection.

So the window `[collect :869, store lock :911]` admits new incident relations
from any concurrent writer, each deleted with **zero authorization**. Postgres
transactionality does not help — `READ COMMITTED` sees rows committed after the
tx began, and **the transaction does not span the authorization**. The earlier
"pg is genuinely transactional anyway" rationale is a non-sequitur; DELETE IT.

Attack: a low-privilege principal races `CreateRelation(victim, sensitive-rel, X)`
against a cascade `DeleteEntity(victim)` by a principal lacking delete on
`sensitive-rel`. Pre-flight never sees the edge; the store deletes it.

**Fix (preferred):** move collect + authorize + delete inside `store.Store.Tx` —
the sanctioned seam (DEC-8UIL0), giving mutual exclusion on fs/mem and snapshot
isolation on pg. **Caveat to verify first:** the ACL evaluator does graph reads
(`resolver.go:220`); confirm they don't route through the outer store handle or
fsstore deadlocks (`fsstore/tx.go:22-27` warns about exactly this).
**Fallback:** re-verify `res.DeletedRelations` against the authorized set after
the fact and fail loudly + audit-flag any difference. Strictly weaker (the
deletion already happened) but converts a silent hole into a detectable one.

### DR-2 (F5/E2/E3) — B3's "abort the cascade and surface the error" is NOT IMPLEMENTABLE as specced.

Stated three times in this ticket and the plan. Neither half works today:

**(a) The runner cannot abort.** `applyRelationCreations` swallows every
`WriteRelation` error into a string and `continue`s
(`autocascade/runner.go:231-234` — VERIFIED), same at `:363-367`.
`Runner.Process` returns non-nil error ONLY for a nil trigger (`:68-70`); the
BFS loop has no error path.

**(b) The error is discarded on 3 of 4 transports.** `outcome.Errors` folds into
`result.AutomationErrors` (`manager.go:568,837`) and the write returns nil.
Complete list of non-test consumers (VERIFIED by grep): `cli/create.go:71`,
`cli/update.go:105`. **`internal/dataentry`, `internal/mcp` and `internal/lua`
have ZERO references.**

So B3 as written produces, on SPA/MCP/**Lua**: HTTP 200 / success, edge silently
absent, no error anywhere. That is a **bit-for-bit reproduction of the motivating
outage** — on the same Lua transport — caused by the fix.

**(c) The `allow_acl_bypass` escape hatch has no data path.** `AllowACLBypass`
lives on `metamodel.AutomationAction` (`types.go:656`) and reaches Lua only via
`automation.LuaToExecute`. The declarative `create_relation:` action emits
`Result.RelationsToCreate []*entity.Relation` — **a bare slice with no action
provenance**. By `WriteRelation` the originating action is unrecoverable.
Threading it changes `automation.Result`, `autocascade.Runner`, and the
`autocascade.Host` consumer interface (`host.go:65`) across 4 packages. Without
it **B3 is breaking with no opt-out**.

None of `internal/autocascade/runner.go`, `internal/automation/types.go`,
`internal/autocascade/host.go` or the CLI appear in the plan's file list.

**Fix: SPLIT B3 OUT.** See "Revised scope" below.

### DR-3 (F10) — the delegate-X overlap check misses TWO of the three role-conferring mechanisms.

The planned hard error covers only `role_relations[k].RequiresPermission != ""`.
There are **three** ways writing a relation confers roles:

1. `role_relations` — covered. ✅
2. **`MembershipRelation`** (default `member-of`, `policy.go:111`,
   `EffectiveMembershipRelation` at `:243-248` — VERIFIED). The resolver walks
   it for group roles (`resolver.go:90`) and it **need not appear in
   `role_relations` at all** — its own godoc says so (`policy.go:395-404`).
   So `relation_grants: {member-of: {create: some-perm}}` with NO
   `role_relations` entry **passes the planned validation cleanly and hands over
   the RR-7O6Q self-promotion primitive verbatim.** Gate A doesn't fire either
   (it keys on `RoleRelations[s.Type]`, `authz_write.go:47`).
3. **`InheritRolesThrough`** (`policy.go:116`). `Request.ancestors`
   (`resolver.go:205-237`) BFS-walks these; `computeForEntity` (`:180-186`)
   confers local roles across the chain. **No delegate gate exists for these at
   all.** A permission-holder can graft any subtree under any parent.

`aclaudit` A2 does not backstop it: `checkUngatedRoleRelations`
(`tier_a.go:79-101`) iterates `p.RoleRelations` only.

**Fix.** Widen the Validate hard error to reject a `relation_grants` key
matching ANY of: `p.EffectiveMembershipRelation()`,
`slices.Contains(p.InheritRolesThrough, k)`, or a gated `role_relations[k]`.
Three negative tests, one per mechanism. Separately extend `aclaudit` A2 to flag
ungated membership / inherit-through types generally (pre-existing gap this
ticket would make exploitable).

## SIGNIFICANT

### DR-4 (F3) — empty `FromType` becomes an ALLOW. The earlier "robustness improvement" claim is wrong.

Today an empty `FromType` fails closed because no role lists `""` (except `"*"`,
`policy.go:317-320`). Once the relation permission is an alternative satisfier
keyed on `s.Type` (always caller-supplied, always populated), the conjunct
becomes `grantsVerb(...) OR grantsPermission(...)` — so a principal holding ONLY
the relation permission can write an edge whose source **does not exist or is
unreadable**. `CreateRelation`'s later `GetEntity` at `:1100` masks this by
accident of ordering; `DeleteRelation` (`manager.go:1240-1250`) has **no
source-existence check at all**.

**Fix.** Gate the new allow source on a resolved source:
`if s.FromType != "" && r.grantsPermission(attrs, perm)`. Negative test:
principal holds only the relation permission, source absent ⇒ DENIED. Same for
`DeleteRelation`. Document empty `FromType` as a fail-closed sentinel.

### DR-5 (F7) — incoming-side renumber mutates relations from unauthorized sources. Remove "Fine." from the B5 row.

`runRenumberAfterUpdate` fires an **incoming** query
`{To: to, Type: relType}` (`manager_order.go:222`) selecting relations of that
type pointing at `to` **from arbitrary other source entities of arbitrary
types**, then writes each via `Store.UpdateRelation` directly, deliberately
bypassing authz (`:186-202`).

After this change a principal can hold `relation_grants: {R: {update: reorder-R}}`
and **no entity-type update grant at all**, then one authorized PATCH on their
own edge triggers writes to `A --R--> shared`, `B --R--> shared` … Bounded to the
order property, but order is semantically meaningful and the audit rows attribute
it to a principal who demonstrably lacks authority.

**Fix.** Authorize the incoming-side renumber plan entries before applying —
`manager_order.go:180-182` is already two-phase, so it is a loop over `plan`.

### DR-6 (F6) — B3 gates the relation TYPE, never the TARGET.

`targetID := e.interpolate(action.CreateRelation.To, event)`
(`automation/engine.go:285-289`) expands `{{new.<prop>}}` — so a low-privilege
user setting a property on the trigger entity **chooses the `To` endpoint**.
`RelationSubject` has no `ToID` (RR-F9M9, deliberate). Do NOT re-litigate that;
**state the residual explicitly**: the block gates type, not target. Compounds
with DR-3 where the type is role-conferring.

### DR-7 (A1) — `decideFromAttrs` must not become a two-mode function.

Threading a relation-only param into a helper shared with
`authorizeEntityWrite` (`authz_write.go:43`) is the "this function does two
things" smell. **Extract `ceilingDenial(attrs, op, target) *Decision`** instead;
`authorizeRelationWrite` then reads linearly with its allow-sources visible at
the call site, and `decideFromAttrs` keeps its exact signature (entity path
byte-identical). This also makes a real subtlety visible: **the ceiling keys on
`FromType` while the grant keys on `s.Type`**, so `deny_write: ["*"]` denies but
`deny_write: [person]` does not deny a `spawnt` edge from `terugkerend`. Correct,
but it must be a commented decision, not an accident.

### DR-8 (B1-naming) — `RelationGrantDef` collides with the existing `acl.RelationGrant`.

Three characters apart, **same package**, genuinely different semantics
(default-permissive affordance hint vs universal write allow-source). Worse than
the YAML collision, since Go has no nesting to disambiguate.
**Use `RelationWriteGrant` for the Go type, `relation_grants:` for the YAML key**
(`ClientBaseline`/`client_baselines` already differ in shape). Cross-reference
both godocs.

### DR-9 (E1) — AC3, "the critical regression test", would PASS VACUOUSLY as written.

`World.principalFor` (`internal/acl/testutil_test.go:171`) builds a plain
principal with no `PrincipalType`, so `compiledCeiling` is **inactive** — the
test would assert "denied" while the ceiling was never engaged, and keep passing
after a regression that broke it. Same vacuity class as the guard test's
`scanned < 5` floor.

**Fix.** Add `World.As(principalType, scopes...)` building via
`principal.VerifiedFrom`, or write AC3 directly against `NewDeclarative` +
`verifiedClient` (`ceilingguard_test.go:186`). **Either way add a positive
precondition assertion** — assert the principal WOULD be allowed without the
ceiling, so a vacuous pass is impossible.

### DR-10 (H1/F11) — no CLI verification surface; the incident's SECOND root cause survives.

This ticket names two causes: no edge verb, and **"silent under every static
check"**. The plan fixes the first and defers the second. `internal/aclmap` has
**zero** `acl.RelationSubject` references; `Can` is entity-shaped throughout
(`can.go:52-61`). So an operator writing `relation_grants:` has **no way to
verify it** short of attempting the write in production — and the ticket adds one
more invisible allow source.

**Fix (pull into scope).** A thin `rela acl can --relation <type> --from <id>
<verb>` that builds a `RelationSubject` and calls `AuthorizeWrite` directly,
printing `RuleKind`/`RuleID`/`Reason`. It IS the runtime path, so it cannot
drift, and needs no `AccessRoutes` extension. Route-level explanation can follow.

### DR-11 (C1) — the `RelationGrant` overlap needs the contract test, not just prose.

The ticket already says "adding one is in scope" but it appears in **no
acceptance criterion**. Verb vocabularies don't even agree (`remove` vs
`delete`). An operator writing both gets **allowed via CLI/Lua/MCP, denied via
SPA**, with neither block wrong.

**Fix.** (1) Add the AC: `RelationVerdicts.Creatable == false` ⇒ the relation
write 403s. (2) Two-sentence composition rule in docs + godoc. (3) File the
convergence follow-up (derive `RelationGrant` verdicts FROM the write gate) so
"document it" isn't the permanent answer.

## MINOR / NIT (fold in during implementation)

- **DR-12 (A2):** allow Decision needs `RuleKind: "relation-grant"`, `RuleID:
  <permission>` — otherwise the audit cannot distinguish "allowed by source-type
  grant" from "allowed by relation permission". That IS the observability fix.
- **DR-13 (N1):** when a `relation_grants` entry exists for `s.Type`, extend the
  deny `Reason` to name the permission that would have satisfied it. Directly
  addresses the "silent under every static check" root cause.
- **DR-14 (N2):** a B3 denial must record a denied-write audit row;
  `cascadeHost.recordCascade` (`cascadehost.go:248-265`) has no denial branch.
  Add it to `cascadeHost` — do NOT give it a `*Manager` back-reference (that
  re-creates the elevation-propagation hazard `gated()` prevents,
  `manager.go:98-103`).
- **DR-15 (F2):** `CreateRelation`/`UpdateRelation` authorize on `fromType` from
  `:1085` then re-fetch at `:1100`. Reuse the single fetch so authorized type
  and validated type are provably identical.
- **DR-16 (F8):** state that the coarse `update:` gates WHETHER a relation may be
  updated; `RelationGrant.Fields` gates WHICH meta-fields, dataentry-path only.
- **DR-17 (F9):** keep shorthand mutual-exclusivity, but the error must name the
  exact expansion so operators can copy-paste the fix — a failed policy load on a
  running deployment is an outage.
- **DR-18 (D1):** reject `read:` with a DISTINCT explanatory error (not "unknown
  verb"). `read:` is not meaningless — it is coherent and deliberately
  unsupported. That error message is the highest-leverage documentation in the
  ticket.
- **DR-19 (F2-plimsoll):** VERIFIED — no `//plimsoll:` directives in
  `internal/acl`. `Policy` 12→13 exported fields (limit 20), `Request` 24→25
  methods (limit 40). Comfortable. Do NOT add a directive to a compliant type.
- **DR-20 (E4):** assert AC8's no-partial-write property on **memstore**, not
  only under `RELA_TEST_DATABASE_URL` — otherwise it is unasserted in default CI.
- **DR-21 (F2-audit):** dedupe B1's pre-flight by `(relationType, op)` — a denied
  5,000-edge cascade otherwise writes 5,000 audit rows.
- **DR-22 (H2):** `slog.Info` at load when `relation_grants:` is non-empty. The
  version-skew footgun ("old binary warn-and-ignores") is otherwise
  indistinguishable from "active but granting nothing".
- **DR-23 (H4):** add `Description` to the new type for the `rela docs`
  generator, per the `Policy.Description` / `RoleDef.Description` precedent.
- **DR-24 (H5):** state the SPA impact — no change needed; the SPA may render an
  add-relation control the write then 403s (attempt-and-recover, accepted in
  Reframe #2).
- **DR-25 (H7):** close open question 5 (answered in B2) and **pin the
  `OpRename` unreachability with a test**, so a future `RelationSubject` +
  `OpRename` caller can't silently inherit `Update` semantics.
- **DR-26:** AC numbering runs 1-7, 9, 10, 8. Renumber.

## Revised scope

**This ticket:** `relation_grants:` block + `authorizeRelationWrite` seam
(DR-7 shape) + validation incl. the widened role-conferring check (DR-3) +
empty-FromType guard (DR-4) + ceiling regression test (DR-9) + **B1 done
transactionally (DR-1)** + renumber fix (DR-5) + **CLI verification (DR-10)** +
contract test (DR-11) + docs.

**Follow-up A (file now, blocks nothing):** carry `allow_acl_bypass` through the
declarative `create_relation:` action — the DR-2(c) plumbing, useful alone.

**Follow-up B (depends on A):** B3 — gate `cascadeHost.WriteRelation` as the
triggering principal, INCLUDING the `autocascade.Outcome` fatal-error contract
change and propagation to dataentry/MCP/Lua. ACs 9 and 10 move here.

Rationale for the split: B3 needs nothing from this ticket (it has no ACL call
today, so gating it is valuable against the CURRENT source-type gate), and this
ticket is fully useful without B3 (the motivating incident was a *Lua*
`create_relation`, already gated). Rollback is asymmetric — the block is additive
and inert until configured; B3 breaks every automation-using deployment with no
opt-out until Follow-up A lands. And a reviewer with ACL expertise and one with
cascade expertise are different people.

**Effort: this ticket stays `l`** (B1-under-Tx and the CLI surface offset B3
leaving). Follow-up B is `m`-to-`l` on its own.

---

# Original ticket (pre-2026-08-19)

Retained for the reframe history and the (a)/(b) reasoning.

## Original goal

Extend ACL v0's `WriteRequest{Op, EntityType, RelationType}` so it can represent
**parameterised verbs** that ACL v0's enum-of-4 can't. The original sketch (from
phase 1) was `transition:<state>` and `relation:<type>:add/remove`.

## Reframes

### Reframe #1 — `transition:` is not the right primitive (2026-05-21)

**User feedback:** "transition as currently specced makes no sense, status is
nothing special; so this should be possible for all enum fields right?"

Correct. The status property has no special status in the metamodel. A generic
`set-prop:<prop>:<value>` would cover any enum.

But the follow-up exposed the limit: "How would set-prop work with enum-list
fields? Or fields of other types?" Per-value gating only works for properties
with a discrete finite value set. Strings, numbers, dates, refs, markdown
content have unbounded cardinality; enum-lists are combinatorial. The verb
taxonomy explodes or collapses.

### Reframe #2 — fine-grained ACL probably belongs in Lua (2026-05-21)

**User insight:** "it feels like the prop/rel level stuff might be better
handled via lua."

Right framing. rela already runs Lua at write time (the automation engine) and
CLAUDE.md explicitly bans Lua on the read path for performance reasons.
Fine-grained ACL maps onto this existing shape cleanly:

| Concern | Where |
|---|---|
| Coarse "can write entity type X" | `acl.yaml` (declarative, unchanged) |
| Field-level "can set status=done" | Lua veto hook on write |
| Field-level "can change priority" | Lua veto hook |
| "Only assignee can mark done" | Lua veto hook |
| Relation add/remove gating | Lua veto hook (or wire-level verb, see below) |

The split lets `acl.yaml` stay fast + declarative (the 90% case) and pushes
programmable predicates into Lua where they belong (read entity properties,
consult the graph, check principal attributes — the kind of context-aware logic
Lua is for).

**Implications for `_actions` (TKT-Y72A's wire shape):**

- `_actions` stays coarse-grained — the existing 4 verbs cover the
declarative cases. No per-property / per-value verb explosion.
- The SPA renders fine-grained controls (dropdowns, buttons)
optimistically; the server's Lua hook decides on write; on deny, the existing
403 error path shows a toast with the Lua-supplied reason. This is the **Stripe
attempt-and-recover pattern** (research §8) — selectively, where the alternative
is verb- cardinality hell.
- UX cost: no "grey out the dropdown option" for fine-grained
cases; only-allowed users click and succeed, others click and see a 403 toast.
Acceptable trade for the architectural cleanliness.

### Reframe #3 — option (a), concretely (2026-08-19)

Answered by the incident above: (b) is not sufficient, because the failure mode
is not "the gate is too coarse to express my rule" but "the gate is attached to
the wrong noun, and no tool can see it". Lua vetos can only *narrow*; they
cannot grant edge-creation to a principal the type gate already denied. Option
(a) it is — see **Design** at the top.

## What this ticket might still cover

Two viable scopes, both smaller than the original:

**(a) Wire-level relation verbs only.** Add `relation:<type>:add` /
`relation:<type>:remove` to the verb vocabulary, gated by an `acl.yaml`
extension that lists allowed relation types per role. The relation widgets
(RelationCards, RelationPicker) hide their add/remove buttons accordingly. Same
pattern as phase 1 for entity-CRUD; just one more verb family. Lua hooks
complement this for "which targets are allowed."

**(b) Drop entirely.** Lua write-veto hooks cover the whole space including
relations; the SPA renders relation buttons optimistically and surfaces 403
toasts from Lua-supplied reasons. TKT-Y72A's `_actions` stays at 4 verbs
forever. The verb taxonomy never grows.

Choice between (a) and (b) is the central design question. (a) is more work
(`WriteRequest` extension + policy schema + SPA gating); (b) is simpler but
accepts attempt-and-recover for relations.

**→ Resolved 2026-08-19 in favour of (a).**

## Prerequisite: Lua write-veto hook (new ticket)

**No longer blocking** — option (a) needs no Lua. Retained as context; a Lua
write-veto hook remains independently useful for "which targets are allowed"
style rules that complement this ticket.

This work depends on a Lua hook the automation engine doesn't yet have:

- Lua script returns `{allow=false, reason="..."}` (or analogous)
from a `pre-write` hook
- `entitymanager.Manager` invokes the hook before the write, audits
the deny, returns `*acl.ForbiddenError` with the Lua-supplied reason
- New `acl.yaml` field (or `metamodel.yaml`? — design choice)
registers per-type or per-relation Lua scripts as write vetos

A separate ticket will spec this. Without it, this ticket can't proceed.

## Out of scope (unchanged from earlier draft)

- ACL v1 per-row rules from `acl.yaml` (separate ticket; the Lua
hook may make this entire direction moot).
- Read-side filtering / property redaction.
- Snapshot threading through `AuthorizeWrite`.

## References

- Phase-1 implementation: TKT-Y72A, PR #779
- Phase-2 implementation: TKT-LFT2, PR #786
- Design: `.ignored/action-affordances-design.md`
- Research: `.ignored/action-affordances-research.md` §8 (Stripe
attempt-and-recover)
- ACL v0: TKT-GN5LN
- Reframes driven by user feedback, 2026-05-21 and 2026-08-19 sessions
