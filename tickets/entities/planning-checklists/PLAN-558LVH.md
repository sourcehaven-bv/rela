---
id: PLAN-558LVH
type: planning-checklist
title: 'Planning: ACL relation permissions (design record for the TKT-K2VN9D / VR61XC / 8HDPQW increments)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

> **This plan covered TKT-XZEY, which was SPLIT INTO INCREMENTS after design
> review.** It is retained as the shared planning record. Each increment ticket
> carries its own scope, semantics and acceptance criteria; the per-increment
> ACs supersede the consolidated list that was here. See TKT-XZEY for the split
> table and DR-1..DR-26.

Relation writes gate on the verb grant for the **source entity's type**
(`authz_write.go:60-64`), so granting edge-creation requires granting
entity-creation on that type. No way to express least privilege; no tool can ask
a relation-shaped question. Caused a production outage (unbounded duplicate
growth → OOM).

**Scope, as split:**

| Increment | Ticket | Effort |
|---|---|---|
| 1 — `relation_grants:` block: config, validation, `authorizeRelationWrite` seam | TKT-K2VN9D | m |
| 2 — `rela acl can` relation verification | TKT-VR61XC | s |
| 3 — cascade delete under `Store.Tx` (B1) | TKT-8HDPQW | m |
| incoming-renumber authz + `OpRename` pin | TKT-JW03LN | s |
| `allow_acl_bypass` plumbing | TKT-7QM4RB | m |
| automation gating (B3) + autocascade fatal-error contract | TKT-M3W8PK | l |

OUT of the whole arc:
- `read:` on relations — endpoint-derived visibility; an independent grant would
  be an entity-existence oracle.
- `TransitionDef.Guard` fix → TKT-505CJ2.
- The non-atomic multi-step Lua write problem (separate, unfiled).
- `RelationGrant` convergence (deriving affordance verdicts FROM the write gate)
  → follow-up, see DR-11.

**Why this split.** Increment 1 is independently shippable and fixes the
motivating incident (a **Lua** `create_relation`, already gated). Increment 2 is
not optional: the incident had two root causes and increment 1 alone fixes only
the first, while adding one more allow source no tool can see. B3 was split
because its true cost is 3-5× the original estimate — it needs an
`autocascade.Outcome` fatal-error contract change plus cross-package bypass
plumbing — and because its rollback profile is opposite to the rest (the config
block is additive and inert until used; B3 breaks every automation-using
deployment until TKT-7QM4RB lands).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — two targeted security/coverage traces run instead
(recorded in full on TKT-XZEY). A `/research` doc would duplicate them.

**Existing Solutions:**

- **Prior art in-tree (the model to copy):** `RoleRelationDef.RequiresPermission`
  (`policy.go:423`) is already a per-relation-type permission gate, checked at
  `authz_write.go:48-58`. The new block is its generalization from
  role-conferring relations to all relations.
- **Permission vocabulary already exists:** `holdsPermission` →
  `grantsPermission` (`resolver.go:267-289`), granted by `RoleDef.Permissions`.
  Routing through it inherits BOTH ceiling axes (`permitsPermission` at `:275`,
  `roleFor` at `:280`) for free.
- **`RelationGrant` (`policy.go:353`) is NOT reusable as the gate** — consumed
  only by `internal/affordances` (`resolver.go:243`), enforced only at the
  dataentry HTTP layer (`affordances.go:560,605,680`), and **default-permissive**
  (`affordances.go:568`). Not Lua/MCP/CLI/sync/automations. Relationship to the
  new block must be documented explicitly.
- **Validation precedents:** blank-key rejection (`policy.go:665-669`),
  scope_grants deny-spelling rejection (`ceiling.go:318-326`), normalize pass
  (RR-IK355A, `policy.go:544-563`), key parity test (`policy_parity_test.go`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** *(revised post-review — DR-7)*

The decision today is a conjunction:

```
allow = (delegate-X satisfied OR not configured)   # gate A, authz_write.go:48-58
      AND ceiling permits verb on FromType          # decideFromAttrs :87
      AND some role grants verb on FromType         # :99-107
```

**The new block may only widen the THIRD conjunct.**

**DR-7 correction:** do NOT thread a relation-only parameter into
`decideFromAttrs` — it is shared with `authorizeEntityWrite` (`:43`) and would
become a two-mode function. Extract `ceilingDenial(op, target, attrs) *Decision`
instead, so `authorizeRelationWrite` reads linearly with its allow sources
visible at the call site and `decideFromAttrs` keeps its exact signature (entity
path byte-identical). Concrete shape in TKT-K2VN9D.

**DR-4:** the new allow source MUST be gated on `s.FromType != ""` — otherwise
"source unresolvable ⇒ deny" silently becomes "⇒ allow".

The permission check MUST be expressed as `r.grantsPermission(attrs, perm)` —
never a direct `policy.Roles[...]`/fresh traversal. This is both the ceiling
requirement and what satisfies `ceilingguard_test.go` for free.

Keep the check on `r.Globals(ctx).Attributions` (as today, `:63`) — never
`computeForEntity`. Sourcing from local roles would be the delegate-X inversion.

**Rejected alternatives:**
- *"Permission is sufficient to authorize the write"* (the original framing) —
  breaks delegate-X **and** the client ceiling. See TKT-XZEY security review.
- *Check at the top of `authorizeRelationWrite`* — natural-reading spot for a
  sufficient check, bypasses gate A. Rejected.
- *`requires_permission` on `metamodel.RelationDef`* — puts a policy claim in
  `schema.yaml`, which is loaded on paths with no ACL. That is the
  `TransitionDef.Guard` bug (TKT-505CJ2). Rejected.
- *Promote `RelationGrant` to the write gate* — closed-world-per-type vs the
  union semantics of `decideFromAttrs`; would re-litigate the affordance
  contract the SPA depends on. Rejected.

**Files to modify:**
- `internal/acl/policy.go` — new type + `Policy` field, `knownPolicyKeys`,
  `Validate` checks, normalize pass
- `internal/entitymanager/manager.go` — cascade delete under `Store.Tx`
  (**TKT-8HDPQW**; the original "pre-flight loop" shape was WRONG, see DR-1)
- `internal/aclmap/` + `internal/cli/acl_can.go` — relation verification
  (**TKT-VR61XC**)
- `internal/entitymanager/manager_order.go` — incoming-renumber authz
  (**TKT-JW03LN**)
- `internal/automation`, `internal/autocascade`, `internal/entitymanager/cascadehost.go`
  — B3 (**TKT-7QM4RB** then **TKT-M3W8PK**)
- `internal/acl/authz_write.go` — additional allow source after the ceiling test
- `internal/acl/ceilingguard_test.go` (or a new guard test file) — ceiling
  regression assertion
- `internal/acl/policy_test.go`, new `authz_relations_test.go`
- `internal/aclaudit/tier_a.go` (dead-permission) and/or `tier_b.go`
  (undeclared relation type)
- `internal/acl/` `MetamodelView` + `internal/appbuild/appbuild.go:815-831` —
  iff the undeclared-type check is a boot error
- `docs/acl-overview.md`, `docs/acl-security.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Sole input is operator-authored `acl.yaml` (trusted-but-fallible; not attacker
input). Validation is about catching operator mistakes that fail *open*:

| Input | Validation | On invalid |
|---|---|---|
| relation-type key | non-blank after trim | hard error at load |
| permission name | non-blank after trim | hard error at load |
| verb key | allowlist `create`/`update`/`delete` | hard error (`read:` is meaningless here) |
| relation type ∩ gated `role_relations:` | must not overlap | **hard error** — ambiguity resolves to escalation |
| relation type declared in metamodel | `ValidateAgainstMetamodel` or `aclaudit` | boot error or audit finding (decide) |
| unknown top-level key | `knownPolicyKeys` | warn-and-ignore (**existing** behaviour — version-skew footgun, document) |

**Security-Sensitive Operations:**

This ticket *is* a security-sensitive operation — it adds an allow path to the
write authorization gate. Three invariants it must not break, each with a test:

1. **Client ceiling only narrows** (`ceiling.go:13-25`). `filterTypes`
   (`ceilingcompile.go:286-292`) deliberately preserves a role's `"*"` under a
   pure denial, so `roleFor` alone does NOT clamp — `permitsVerb` at
   `authz_write.go:87` is what actually denies. Verified by direct read.
2. **Delegate-X / RR-7O6Q** (`policy.go:394-421`). Gate A must stay first.
3. **`roleFor` is the single clamp point** (`ceilingguard_test.go`). Its regex
   matches only `policy\.Roles\[` — a new `policy.Relations[` allow-path is
   **invisible to it**. Positive regression test required; new file must NOT be
   added to `exemptFiles`.

Error handling: denials reuse the existing structured `Decision`
(`RuleKind`/`RuleID`/`Reason`). Permission and relation-type names are
operator-authored config, not secrets (CLAUDE.md: "The configuration is not a
secret; the data is"), so naming them in a 403 is correct and useful.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** *(consolidated; the authoritative per-increment ACs live on
each increment ticket. Two entries below were CORRECTED at design review.)*

| AC | Test |
|---|---|
| 1 | Integration: outage config verbatim (`create: [taak]`, `update: [taak, terugkerend]`, new block grants `spawnt` create) → `CreateRelation` ALLOWED |
| 2 | Same policy → `CreateEntity(terugkerend)` DENIED |
| 3 | **`deny_write: ["*"]` ceiling + principal holds relation permission → ALL DENIED, `RuleKind: "client-ceiling"`.** ⚠ **DR-9: would pass VACUOUSLY as originally written** — `World.principalFor` (`testutil_test.go:171`) builds an unverified principal, so the ceiling never activates. Needs `World.As(principalType, ...)` via `principal.VerifiedFrom`, or `NewDeclarative` + `verifiedClient`, **plus a positive precondition assertion** (allowed without the ceiling). |
| 4 | `role_relations: {member-of: {requires_permission: delegate-membership}}` + new block granting `member-of` create; principal lacks delegate → DENIED with `RuleKind: "delegate-permission"` |
| 5 | **THREE** overlap boot errors (DR-3), not one: `role_relations`, `EffectiveMembershipRelation()`, `InheritRolesThrough`. Checking only `role_relations` misses the RR-7O6Q path verbatim, since `member-of` need not appear there. |
| 6 | Full existing `internal/acl` + `internal/dataentry` suites green unchanged |
| 7 | `aclaudit` finding for a relation permission no role grants |

**Edge Cases:**

- **Empty `FromType` — CORRECTED (DR-4).** 4 of 5 construction sites leave it
  empty when the source is missing (`manager.go:1090,1174,1244`,
  `affordances.go:66`). The earlier claim that a `s.Type`-keyed check is
  "unaffected" was WRONG: in the CONJUNCTION, an unresolvable source stops
  having to satisfy anything, turning deny into allow. Requires the
  `s.FromType != ""` guard **and** a negative test (only the relation
  permission held, source absent ⇒ DENIED), including for `DeleteRelation`,
  which has no source-existence check at all (`manager.go:1240-1250`).
- Relation type in the new block but absent from the metamodel.
- Both shorthand and an explicit verb present → mutually exclusive, `Validate` error.
- Empty block (`relation_grants: {}`) → valid, inert.
- `read:` under an entry → distinct explanatory error, not "unknown verb" (DR-18).
- Cascade delete racing a concurrent `CreateRelation` (DR-1) → never deleted
  unauthorized. **TKT-8HDPQW.**
- Whitespace-padded keys/permissions → normalized then rejected if blank.
- Permission held via the `everyone` role.
- Wildcard `"*"` as a permission name under `deny_permissions`
  (`filterPermissions` preserves it, `ceilingcompile.go:314-317`).
- `ApplyRelation` create-vs-update resolution (`apply.go:237-241`) — both branches.

**Negative Tests:**

Blank relation key; blank permission; unknown verb key; overlap with gated
`role_relations:`; ceiling denial (AC3); delegate denial (AC4); principal holding
a *different* relation's permission; permission held but ceiling
`deny_permissions` withholds it.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| Implementer follows the *word* "sufficient" and short-circuits → delegate-X + ceiling both bypassed | **critical** | Word purged from the ticket; semantics restated as "alternative satisfier of gate B only"; AC3/AC4 tests |
| `ceilingguard_test.go` blind spot (regex is `.Roles[` only) lets a future `policy.Relations[` path escape silently | **high** | Positive ceiling regression test (AC3); consider widening the regex |
| **Cascade delete TOCTOU (DR-1)** — the "pre-flight" authorizes a snapshot taken outside any lock; both stores re-derive the set inside their own lock/tx (`fsstore/entity.go:308-313`, `pgstore/entity.go:371-382`) | **critical** | The original mitigation was WRONG. Must run collect+authorize+delete under `store.Store.Tx`. **TKT-8HDPQW**, with a blocking spike on ACL graph reads vs fsstore re-entrancy (`fsstore/tx.go:22-27`). |
| **B3 cannot abort (DR-2)** — `runner.go:231-234` swallows errors and `continue`s; `AutomationErrors` has only 2 consumers (`cli/create.go:71`, `cli/update.go:105`), **none** in dataentry/MCP/Lua | **critical** | Naive B3 reproduces the outage on the Lua path. Split → **TKT-M3W8PK** with an `autocascade.Outcome` fatal-error contract. |
| **Role-conferring overlap check too narrow (DR-3)** — `member-of` need not appear in `role_relations`; `InheritRolesThrough` has no delegate gate | **critical** | Widen to all three mechanisms. **TKT-K2VN9D AC5.** |
| **Empty `FromType` becomes an allow (DR-4)** | significant | `s.FromType != ""` guard + negative test. **TKT-K2VN9D AC6.** |
| **AC3 would pass vacuously (DR-9)** | significant | Verified principal + positive precondition assertion. |
| **Incoming-side renumber (DR-5)** writes relations from unauthorized sources | significant | **TKT-JW03LN.** Previously dismissed as "Fine" — wrong for the incoming query. |
| **No CLI verification (DR-10)** — the incident's second root cause survives, and increment 1 adds an invisible allow source | significant | **TKT-VR61XC**, pulled into the arc rather than deferred. |
| B3 migration break — automations start failing when the triggering user lacks the grant | high | **TKT-M3W8PK**; escape hatch needs **TKT-7QM4RB** first, else there is no opt-out. |
| Two overlapping YAML surfaces (`RelationGrant` vs new block) confuse operators; verb vocabularies don't even agree (`remove` vs `delete`) | medium | Contract test as an AC (DR-11), two-sentence composition rule, and a filed convergence follow-up — not just "document it" |
| Naming collision — YAML **and Go** (DR-8): `acl.RelationGrant` already exists in the same package | medium | YAML `relation_grants:`; Go type **`RelationWriteGrant`** (not `RelationGrantDef` — three chars from the existing type). Cross-reference both godocs. |
| Version skew: older binary warn-and-ignores the block (`policy.go:465-471`) | medium | Fails closed, but document loudly |
| Dataentry soft-condition fallback (`relations_modern.go:393,434`) writes to the store directly | low | Safe **iff** enforced inside `authorizeRelationWrite`; constrains placement |

**Effort (revised, split):** TKT-K2VN9D `m` · TKT-VR61XC `s` ·
TKT-8HDPQW `m` · TKT-JW03LN `s` · TKT-7QM4RB `m` · TKT-M3W8PK `l`.

The original single-ticket `l` understated B1 (which needs `Store.Tx`, not a
loop) and B3 (3-5×: an `autocascade` contract change plus cross-package
plumbing).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/acl-overview.md` — the new block; disambiguate from role-scoped `relations:` (`:456`)
- [x] `docs/acl-security.md` — ceiling/delegate interaction; **document the
      source-type gating, which is currently undocumented** (a root cause of the
      incident: nothing surfaced the requirement)
- [x] `CLAUDE.md` — the "gate declarations live in acl.yaml, not schema.yaml" rule
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change unless the boot-error option is taken)
- [x] `docs/cli-reference.md` — `rela acl can` relation verification (TKT-VR61XC; pulled into the arc, no longer a deferral)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (2026-08-19):** two independent reviews
(adversarial-security + design/completeness), converging on three criticals.
Full findings DR-1..DR-26 recorded on **TKT-XZEY**.

**Three criticals invalidated load-bearing claims in this plan** — all corrected
in place on the ticket:
- **DR-1** — B1's "pure pre-flight ⇒ no race" is WRONG (both stores re-derive
  the relation set inside their lock/tx). B1 must run under `store.Store.Tx`.
- **DR-2** — B3's "abort the cascade" is not implementable in the current runner,
  and `AutomationErrors` is dropped on dataentry/MCP/**Lua**. **B3 split out.**
- **DR-3** — the delegate-X overlap check missed `MembershipRelation` and
  `InheritRolesThrough`, reinstating RR-7O6Q verbatim.

Significant: DR-4 (empty `FromType` becomes an allow), DR-5 (incoming renumber),
DR-6 (target not gated), DR-7 (seam shape), DR-8 (Go type name collision),
DR-9 (**AC3 would pass vacuously**), DR-10 (no CLI verification — the incident's
second root cause), DR-11 (contract test missing from the ACs).

**Status: findings addressed via the split (2026-08-19).** Every DR finding is
either folded into this plan or carried by the increment that owns it:

| Finding | Where it now lives |
|---|---|
| DR-1 cascade TOCTOU | TKT-8HDPQW (redesigned around `Store.Tx` + a blocking spike) |
| DR-2 B3 cannot abort / no bypass path | TKT-M3W8PK (+ TKT-7QM4RB prerequisite) |
| DR-3 role-conferring overlap too narrow | TKT-K2VN9D validation #3, AC5 |
| DR-4 empty `FromType` | TKT-K2VN9D seam guard, AC6 |
| DR-5 incoming renumber | TKT-JW03LN |
| DR-6 target not gated | TKT-M3W8PK "residual, deliberately not closed" |
| DR-7 seam shape (`ceilingDenial`) | TKT-K2VN9D implementation shape |
| DR-8 Go type name collision | TKT-K2VN9D naming |
| DR-9 vacuous AC3 | TKT-K2VN9D AC3 (verified principal + precondition assertion) |
| DR-10 no CLI verification | TKT-VR61XC |
| DR-11 contract test missing from ACs | TKT-K2VN9D out-of-scope note + convergence follow-up |
| DR-12/13/22 observability | TKT-K2VN9D observability section |
| DR-17/18 validation messages | TKT-K2VN9D validation #4/#5 |
| DR-19 plimsoll | verified: `Policy` 12→13 fields (limit 20), no directive needed |
| DR-20 memstore assertion | TKT-8HDPQW AC2 |
| DR-21 audit dedupe | TKT-8HDPQW approach |
| DR-25 `OpRename` pin | TKT-JW03LN |
| DR-26 AC numbering | resolved by the split — each increment numbers its own |

**TKT-K2VN9D is ready to implement.**
