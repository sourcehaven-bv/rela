---
id: RES-VWNN2T
type: research
title: How should the ACL auth-misconfiguration audit be structured (package boundary, finding taxonomy, Validate migration)?
summary: Build the audit as a new internal/aclaudit package (arch-lint forbids acl→metamodel; folding into analysis couples a broad facade to security) taking a narrow consumer-side MetamodelReader; typed Finding with a 5-level severity enum; rela acl audit Kong command with --exit-code; migrate the two membership warns out of Validate. v1 = Tier A+B.
status: done
---

## Problem

`Policy.Validate` is a tolerant-by-design **boot gate**: it hard-errors only on
security-critical structural invariants (`update⊆read`, blank relation keys) and
is policy-internal — it never sees the metamodel or the graph. Three classes of
misconfiguration are therefore invisible today:

1. **Escalation foot-guns** — structurally-valid config that lets a principal
grant themselves power (un-gated membership relation / role-relation, privileged
role on `everyone`).
2. **Dead / inert config** — rules that silently never fire: assignment to an
undeclared role, `confers` an undeclared role, a `requires_permission` no role
grants, a typo'd type name.
3. **Policy-vs-metamodel drift** — grants naming entity types, relation types,
fields, or enum options the schema doesn't define. These fail *silently* today
(the grant just matches nothing).

TKT-Z8A62F bolted two ad-hoc `slog.Warn` hardening checks into `Validate` —
advisory analysis living in a boot gate, the wrong home. This research decides
how to build a proper on-demand audit (`rela acl audit`) and where it lives.

Decisions already taken with the user (inputs to this research, not open):
- v1 = Tier A (pure-policy) + Tier B (metamodel cross-check). Tier C (graph) and
Tier D (reachability) are follow-ups.
- Primary surface = a CLI subcommand `rela acl audit`.
- Audit is advisory; never blocks boot. `Validate` keeps its structural gating.
CI enforcement is opt-in via `--exit-code` (a pipeline choice).
- The two membership `slog.Warn`s migrate OUT of `Validate` INTO the audit.

## Context

**The ACL surface the audit reasons over** (`internal/acl/policy.go`):
- `Policy{UserEntityType, MembershipRelation, Roles, Assignments, RoleRelations,
InheritRolesThrough}`.
- `RoleDef{Create/Update/Delete/Read []string, Permissions []string, Fields/
Visible/Options/Relations maps}` — `"*"` wildcard per verb.
- `RoleRelationDef{Confers, RequiresPermission}` — empty `RequiresPermission`
disables the delegate-X gate (the foot-gun).
- `EveryoneRole = "everyone"` — held by every principal.
- Resolver skips `role_relations` whose `confers` role isn't declared, and
assignments to undeclared roles — i.e. these are *silently inert* today
(confirmed in `resolver.go` computeForEntity / computeGlobals).

**Existing analyzer prior art** (survey, file:line):
- No unified `Finding` base struct. Each analyzer owns its result type:
`analysis.CardinalityViolation`, `validation.Violation{RuleName, Description,
Severity string, EntityID, EntityTitle}`
(`internal/validation/validation.go:15`), `schema.PropertyError`,
`schema.Analysis`. Severity is a **string** (`"error"`/`"warning"`), counted
post-hoc.
- CLI render: unified envelope `output.AnalysisResult{Status, Message, Count,
Details interface{}}` (`internal/output/output.go`), text via `out.Write*`.
- MCP render: text + embedded JSON (`internal/mcp/tools_analysis.go`), not the
envelope.
- `internal/analysis` is the read-only **service facade** both CLI and MCP call;
constructed via `analysis.New(analysis.Deps{Store, Meta, Tracer, ...})`
(`internal/cli/cli_wiring.go:157`).
- **Metamodel cross-check is already a solved pattern**: `internal/validator`
(`New(store.EntityReader, *metamodel.Metamodel, lua.ReadDeps)`) and
`internal/schema/validate_properties.go ValidateEntityProperties(ctx, store,
*metamodel.Metamodel)` both take store + metamodel and cross-check.

**Metamodel reader API the audit needs** (`internal/metamodel`, survey):
- `HasEntityType(t) bool` / `GetEntityDef(t) (*EntityDef, bool)` / `EntityTypes()
[]string`.
- `GetRelationDef(r) (*RelationDef, bool)` → `.From []string`, `.To []string`,
`.Inverse`. Plus `ValidateRelation(rel, from, to) error` — does the B3
source-type-compatibility check in one call.
- Enum options: `EntityDef.Properties[f].Values []string` (inline), or
`Metamodel.Types[prop.Type].Values` (named custom type).
- **Both `Metamodel` (30 exported methods) and `EntityDef` (23) are over the
plimsoll line** — a consumer must take a NARROW interface, not `*Metamodel`.

**Architecture boundaries** (`.go-arch-lint.yml`, confirmed):
- `acl: mayDependOn [entity, principal, store]` — **acl may NOT import
metamodel.** So the audit core (needs both Policy + schema) cannot live in
`internal/acl`.
- `metamodel: mayDependOn [migration, storage]` — leaf; safe to depend on from
above.
- `analysis: mayDependOn [lua, metamodel, project, schema, storage, store,
tracer, validation]` — has metamodel but **NOT acl** today; does not import acl.

**The Validate-migration behaviour change.** Removing the two `slog.Warn`s from
`Validate` means boot no longer logs them; operators see those findings only by
running `rela acl audit`. This is a (small) behaviour change shipped in
TKT-Z8A62F's wake. It is the right move — `Validate` returns to a pure
structural gate — but must be called out and the membership-warn tests in
`policy_test.go` (`TestPolicy_MembershipRelation_UngatedWarns` / `GatedNoWarn`)
must move to the audit's test suite.

## Options

### Q1 — Where does the audit core live?

**Option A: new `internal/aclaudit` package.**
- `aclaudit.Audit(policy *acl.Policy, meta MetamodelReader) []Finding`, where
`MetamodelReader` is a narrow consumer-side interface (HasEntityType,
GetRelationDef, enum-options lookup — 3-4 methods).
- arch-lint: add `aclaudit: { in: internal/aclaudit }` +
`mayDependOn: [acl, metamodel, store]`. No cycle (both below it).
- Pros: security analyzer isolated and independently testable; pure function of
(policy, schema) for Tier A+B (no store needed until Tier C); the narrow
interface satisfies plimsoll and the CLAUDE.md consumer-side-interface rule;
doesn't widen `analysis`'s already-broad dep set with a security concern.
- Cons: one more package; CLI/MCP wiring must construct it (small).
- Effort: S–M.

**Option B: fold into `internal/analysis`.**
- Add `acl` to `analysis.mayDependOn`; add `analysis.AuditACL()` reusing
`analysis.Deps{Meta, Store, ...}` and the `AnalysisResult` render path.
- Pros: reuses existing service wiring + JSON envelope + the CLI/MCP analyze
plumbing; "all analyzers in one place" symmetry.
- Cons: `analysis` becomes the first place that imports `acl`, coupling the
broad analysis facade to the security subsystem; the audit inherits `analysis`'s
heavy `Deps` even though Tier A+B only need policy + a narrow metamodel view;
harder to unit-test in isolation; mixes a security-sensitive concern into a
grab-bag facade (against the "scoped bundle" CLAUDE.md rule).
- Effort: S.

**Option C: extend `internal/acl` with a metamodel-taking method.** Rejected
outright — violates the arch-lint `acl → metamodel` prohibition; would force a
boundary exception on the security package. Not viable.

### Q2 — Finding taxonomy & severity model

**Option A: typed `Finding` with a severity enum (recommended).**
```go
type Severity int // Critical, High, Medium, Low, Nit
type Finding struct {
    Rule     string   // stable ID, e.g. "A1-ungated-membership"
    Severity Severity
    Subject  string   // role / relation / type name the finding is about
    Detail   string   // human explanation
    Fix      string   // one-line remediation
}
```

- Pros: richer than the existing string severity (5 levels lets `--exit-code`
gate on critical/high only); stable rule IDs make findings greppable and
suppressDocumentable; `Fix` mirrors good linter UX.
- Cons: introduces a 5-level enum where the rest of the codebase uses
`"error"/"warning"` strings — minor inconsistency.

**Option B: reuse the string `"error"/"warning"` convention.** Consistent with
`validation.Violation`, but too coarse for "this is a critical self-promotion
path" vs "this is a dead permission nit." The audit's whole value is ranking.
Recommend A but document the divergence.

### Q3 — Validate migration

Single option, already decided: move the two membership warns into the audit as
findings A1 (un-gated membership) / A1b (inert/empty assignments), delete them
from `Validate`, relocate their tests. Confirm no other caller relies on the
boot-time warning (grep: only `policy_test.go`).

## Recommendation

**Q1 → Option A (new `internal/aclaudit` package).** The arch-lint boundary
makes `acl`-resident impossible (Option C) and folding into `analysis` (Option
B) couples a broad facade to the security subsystem and drags in heavy deps the
pure-policy/metamodel checks don't need. A dedicated `aclaudit` package taking a
narrow `MetamodelReader` consumer-side interface is the CLAUDE.md-idiomatic
choice: isolated, plimsoll-safe, unit-testable as a pure function of (policy,
schema) for v1, and ready to take a `store` reader when Tier C lands. The CLI
(`rela acl audit`) and a future MCP `analyze_acl` tool are thin frontends over
it; reuse `output.AnalysisResult` for the CLI JSON envelope.

**Q2 → Option A (typed Finding + 5-level severity enum).** The ranking *is* the
product; `--exit-code` gates on Critical/High. Document the divergence from the
`"error"/"warning"` string convention as deliberate.

**Q3 →** migrate the two membership warns out of `Validate` into `aclaudit`
findings; relocate their tests; note the boot-no-longer-logs behaviour change in
the ticket.

**Tradeoffs accepted:** one new package + a new arch-lint component; a severity
enum that diverges from the codebase's string convention; and a small behaviour
change (boot stops logging the two membership warns). All three are justified by
keeping the security analyzer isolated, rankable, and properly homed.

**v1 check set (for the implementing ticket):**
- Tier A (pure-policy): A1 un-gated membership relation (incl. default
member-of); A1b inert membership (empty assignments); A2 un-gated role-relation
conferring a privileged role; A3 privileged role on `everyone`; A4 assignment→
undeclared role; A5 `confers`→undeclared role; A6 `requires_permission` no role
grants; A7 dead permission; A9 wildcard sprawl on a non-admin role; A10
whitespace/casing in name fields.
- Tier B (metamodel): B1 grant references undeclared entity type; B2
membership/role/inherit relation undeclared; B3 membership relation `from`
incompatible with `user_entity_type` (via `ValidateRelation`); B4 fields/visible
grant names an undeclared field; B5 options grant outside the field's enum; B7
`user_entity_type` undeclared.
- Deferred: Tier C (graph: C1 membership relation has zero edges; C4 privileged
group reachable by everyone) and Tier D (reachability model-check).
