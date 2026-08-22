---
id: TKT-K2VN9D
type: ticket
title: 'ACL increment 1: relation_grants: block — config, validation, and the authorizeRelationWrite seam'
kind: enhancement
status: in-progress
priority: high
effort: m
---

## Increment 1 of 3

Split from **TKT-XZEY** (2026-08-19) so the work lands reviewably. This ticket
is the whole user-visible feature: config → validation → enforcement. It is
independently shippable and independently useful — it fixes the motivating
incident (a **Lua** `create_relation`, already routed through
`EntityManager`, so already covered by `authorizeRelationWrite`).

- **Increment 2** — TKT-VR61XC: `rela acl can` relation verification.
- **Increment 3** — TKT-8HDPQW: cascade-delete authorization under `Store.Tx` (B1).

Read **TKT-XZEY** for the full design rationale, the security review
(DR-1..DR-26), and the two breaks the design must not reintroduce. This ticket
carries only what increment 1 implements.

## Config surface

```yaml
relation_grants:
  spawnt:
    create: create-spawnt
    update: edit-spawnt
    delete: remove-spawnt
```

shorthand (all three verbs):

```yaml
relation_grants:
  spawnt:
    permission: manage-spawnt
```

Values are ACL **permission names** — the existing capability-noun vocabulary
granted by `RoleDef.Permissions`.

**Go naming (DR-8):** type `RelationWriteGrant`, field
`Policy.RelationWriteGrants`, YAML key `relation_grants:`. The type name must
NOT be `RelationGrantDef` — `acl.RelationGrant` already exists in the same
package (`policy.go:353`) with different semantics. Cross-reference both godocs.

## Semantics — the one rule that must not be broken

> A `relation_grants:` permission is an **alternative satisfier of the
> source-type verb grant (gate B) only.** It is NEVER "sufficient to authorize
> the write."

Today's decision is a conjunction (`authz_write.go:46-66`):

```
allow = (delegate-X satisfied OR not configured)   # gate A, :48-58
      AND ceiling permits verb on FromType          # decideFromAttrs :87
      AND some role grants verb on FromType         # :99-107
```

This ticket widens **only the third conjunct**. Gate A and the ceiling stay
unconditional. See TKT-XZEY "Security review outcome" for why "sufficient"
breaks both.

## Implementation shape (DR-7)

Do **not** thread a relation-only parameter into `decideFromAttrs` — it is
shared with `authorizeEntityWrite` (`:43`) and would become a two-mode function.

Extract the ceiling test instead:

```go
// ceilingDenial returns the structured client-ceiling deny when the ceiling
// withholds op on target, or nil when it permits. Extracted so BOTH write
// paths get the clamp in the same position without decideFromAttrs having to
// know which verbs each path can satisfy.
func (r *Request) ceilingDenial(op Op, target string, attrs []RoleAttribution) *Decision
```

`authorizeRelationWrite` then reads linearly:

```go
// Gate A — delegate-X. Unconditional, structurally FIRST. (unchanged)
if rr, ok := r.d.policy.RoleRelations[s.Type]; ok && rr.RequiresPermission != "" { ... }

attrs := r.Globals(ctx).Attributions   // globals only — never computeForEntity

// Gate B ceiling — identical placement/semantics to the entity path.
if deny := r.ceilingDenial(op, s.FromType, attrs); deny != nil {
    return *deny
}
// Gate B satisfier 1: relation permission. REQUIRES a resolved source (DR-4).
if s.FromType != "" {
    if perm, ok := r.d.policy.relationPermissionFor(s.Type, op); ok &&
        r.grantsPermission(attrs, perm) {
        return Decision{Allow: true, RuleKind: "relation-grant",
            RuleID: perm, Attributions: attrs}
    }
}
// Gate B satisfier 2: the pre-existing source-type verb grant.
return r.decideFromAttrs(attrs, op, s.FromType, "no role grants %s on relations from type %q")
```

`decideFromAttrs` keeps its exact signature; the entity path stays
byte-identical.

Three non-negotiables:

1. **`s.FromType != ""` guard (DR-4).** Without it, "source unresolvable ⇒ deny"
   silently becomes "source unresolvable ⇒ allow". An empty `FromType` is a
   fail-closed sentinel, not a wildcard.
2. **Route the permission check through `r.grantsPermission`** — never
   `policy.Roles[...]` or a fresh traversal. It applies `permitsPermission`
   (`resolver.go:275`) and `roleFor` (`:280`), so both ceiling axes are
   inherited for free, and `ceilingguard_test.go` is satisfied by construction.
3. **Globals only.** Sourcing from `computeForEntity` would let a
   locally-conferred role authorize edge creation — the delegate-X inversion.

**Note the deliberate asymmetry, and comment it at the call site:** the ceiling
keys on `FromType` (entity types are its vocabulary) while the grant keys on
`s.Type`. So `deny_write: ["*"]` denies, but `deny_write: [person]` does not
deny a `spawnt` edge from a `terugkerend`. Correct, but must be a recorded
decision rather than an accident.

## Validation — `Policy.Validate` (no metamodel needed)

1. Blank relation-type key → hard error (parallel to `role_relations`,
   `policy.go:665-669`).
2. Blank permission name under any verb → hard error. Silently-inert config is
   the failure mode operators least notice.
3. **Role-conferring overlap → hard error (DR-3, security-critical).** Reject a
   `relation_grants` key matching **ANY** of:
   - `p.EffectiveMembershipRelation()` (`policy.go:243`) — `member-of` need NOT
     appear in `role_relations`, so checking only `role_relations` misses the
     RR-7O6Q self-promotion path verbatim;
   - `slices.Contains(p.InheritRolesThrough, k)` (`policy.go:116`) — these
     confer local roles across a subtree with no delegate gate anywhere;
   - `p.RoleRelations[k].RequiresPermission != ""`.
4. Shorthand + explicit verb → hard error (mutually exclusive). **The message
   must name the exact expansion** so operators can copy-paste the fix (DR-17) —
   a failed policy load on a running deployment is an outage.
5. `read:` → **distinct, explanatory error** (DR-18), not "unknown verb":
   relation visibility is derived (both endpoints visible), so an independent
   read grant would be an entity-existence oracle. This error message is the
   highest-leverage documentation in the ticket. Any other unknown verb → the
   generic error.
6. Add `"relation_grants"` to `knownPolicyKeys` (`policy.go:429-443`) —
   `TestKnownPolicyKeysMatchStruct` (`policy_parity_test.go`) fails until you do.
7. Normalize pass trimming keys and permission names (RR-IK355A precedent,
   `policy.go:544-563`).

## Observability

- **Allow Decision carries `RuleKind: "relation-grant"`, `RuleID: <permission>`**
  (DR-12). Without it the audit cannot distinguish "allowed by source-type
  grant" from "allowed by relation permission" — that IS the observability fix.
- **Deny reason names the permission (DR-13):** when a `relation_grants` entry
  exists for `s.Type`, extend the reason to name the permission that would have
  satisfied it. Directly addresses the ticket's "silent under every static
  check" root cause.
- **`slog.Info` at load when the block is non-empty (DR-22)**, naming covered
  relation types. Unknown top-level keys are warn-and-ignore
  (`policy.go:465-471`), so on an OLD binary the block is silently inert — "the
  log says nothing" must be distinguishable from "active".

## Acceptance criteria

1. Principal holding only `create-spawnt` (no `create:` on the source entity
   type) CAN create `spawnt`. Integration test reproducing the outage config.
2. Same principal still CANNOT create entities of the source type.
3. **`deny_write: ["*"]` ceiling STILL DENIES** the relation write for a
   principal holding the relation permission, `RuleKind: "client-ceiling"`.
   **⚠ DR-9: `World.principalFor` (`testutil_test.go:171`) builds an UNVERIFIED
   principal, so the ceiling is inactive and this test would pass VACUOUSLY.**
   Either add `World.As(principalType, ...)` building via
   `principal.VerifiedFrom`, or write it against `NewDeclarative` +
   `verifiedClient` (`ceilingguard_test.go:186`). **Must include a positive
   precondition assertion** — assert the principal WOULD be allowed without the
   ceiling, so a vacuous pass is impossible.
4. Delegate-X non-regression: principal holds the relation permission on a
   gated `role_relations` type but not the delegate permission → DENIED with
   `RuleKind: "delegate-permission"`.
5. **Three** role-conferring overlap boot errors (DR-3): one each for
   `role_relations`, `EffectiveMembershipRelation()`, `InheritRolesThrough`.
6. **Empty-FromType negative test (DR-4):** principal holds ONLY the relation
   permission, source entity absent → DENIED, not allowed. Same for
   `DeleteRelation` (`manager.go:1240-1250` has no source-existence check).
7. Ceiling-guard companion test: a new `policy.RelationWriteGrants[...]`
   allow-path is invisible to `ceilingguard_test.go`'s `policy\.Roles\[` regex
   (verified) — AC3 is the regression test that closes that blind spot. The new
   file must NOT be added to `exemptFiles`.
8. Shorthand/explicit exclusivity, blank-key, blank-permission and `read:`
   errors all fire with the specified messages.
9. Existing policies (no block) behave **byte-identically** — full existing
   suites green.
10. `aclaudit`: finding for a relation permission no role grants; undeclared
    relation type reported (tier B — decide boot-error vs advisory, see below).

## Undeclared relation types

`Policy.ValidateAgainstMetamodel` (`policy.go:772`) exists and runs as a **hard
boot error** at the wiring site (`appbuild.go:842`) via the narrow
`MetamodelView`. Adding `HasRelationType` to that view is small and
well-precedented (`appbuild.go:815-831`), and matches the existing treatment of
an undeclared `user_entity_type`. **Decide** boot-error vs `aclaudit` advisory
and record it; boot-error is the stronger option.

## Out of scope

- `read:` on relations (see TKT-XZEY).
- CLI verification → TKT-VR61XC.
- Cascade delete → TKT-8HDPQW.
- Automation gating → TKT-M3W8PK / TKT-7QM4RB.
- Renumber incoming-side authorization (DR-5) → TKT-JW03LN.
- `RelationGrant` convergence → TKT-XZEY follow-up.

## Files

`internal/acl/policy.go`, `internal/acl/authz_write.go`, new
`internal/acl/authz_relations.go` (+ `_test.go`),
`internal/acl/policy_test.go`, `internal/aclaudit/tier_a.go` / `tier_b.go`,
possibly `internal/acl/policy.go` `MetamodelView` +
`internal/appbuild/appbuild.go`, `docs/acl-overview.md`, `docs/acl-security.md`.

---

## Implementation notes (2026-08-22)

Branch `tkt-k2vn9d-relation-grants`. All ACs implemented.

### What landed

- **`acl.RelationWriteGrant`** (`policy.go`) + `Policy.RelationWriteGrants`
  keyed `relation_grants:`. Named per DR-8 to avoid colliding with the existing
  `acl.RelationGrant`; both godocs cross-reference each other.
- **Seam** (`authz_write.go`): extracted `ceilingDenial`, so both write paths
  apply the clamp in one place and `decideFromAttrs` keeps its exact signature
  (entity path byte-identical). The relation grant is an allow source checked
  AFTER the ceiling, gated on `s.FromType != ""`.
- **Validation**: blank key/permission, shorthand-vs-per-verb exclusivity (error
  spells out the expansion), unknown verb keys rejected via `UnmarshalYAML`,
  `read:` refused with its own explanatory message, normalize pass, and
  `knownPolicyKeys` entry.
- **Role-conferring refusal** (DR-3): all three mechanisms —
  `EffectiveMembershipRelation()`, `InheritRolesThrough`, and gated
  `role_relations`.
- **Undeclared relation types** → boot error via `ValidateAgainstMetamodel`,
  with `HasRelationType` added to `MetamodelView` (3 adapters updated).
- **Observability**: `RuleKind: "relation-grant"` on the allow;
  `explainRelationDenial` names the permission that would have satisfied a
  denial; `slog.Info` at load confirming the block is active.

### Verification

`go test ./...` green. `golangci-lint` 0 issues. `just arch-lint` OK.
`just plimsoll` OK (`Policy` 12→13 exported fields, limit 20).

**Mutation-tested the two guards that matter**, because both are the kind that
pass vacuously:

1. Removing the `s.FromType != ""` guard → `TestRelationGrant_RequiresResolvedSourceType` FAILS. ✅
2. Moving the relation-grant check above the ceiling (the exact "sufficient"
   mistake DR-7 predicted) → `TestRelationGrant_CeilingStillDenies` FAILS on all
   three verbs. ✅

The second mutation is worth recording: a first attempt at it appeared to pass,
but the script had silently not reordered anything. Re-running it properly
produced the failure. A probe confirmed why the test is necessary — under
`deny_write: ["*"]` the ceiling denies the verb while `grantsPermission`
still returns **true**, so without the ordering the permission alone would
authorize the write.

### Deviations from the ticket

- `permissionFor` returns false for `OpRename` rather than routing through
  Update the way `grantsVerb` does. No caller pairs `OpRename` with a
  `RelationSubject`, so accepting it would invent a semantic no path exercises.
  Pinned by `TestRelationWriteGrant_ShorthandCoversWriteVerbsOnly`.
  (TKT-JW03LN pins the unreachability itself.)
- An **ungated** `role_relations` entry (no `requires_permission`) is still
  grantable — it confers a role, but there is no delegate gate for the grant to
  undercut, and `rela acl audit` A2 already flags that shape as the real
  problem. Pinned by `TestRelationGrants_AllowsOrdinaryRelationTypes`.
