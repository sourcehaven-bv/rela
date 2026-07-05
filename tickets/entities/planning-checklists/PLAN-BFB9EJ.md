---
id: PLAN-BFB9EJ
type: planning-checklist
title: 'Planning: ACL: dedicated authorization-misconfiguration validator / audit insights (escalation foot-guns, dead assignments, un-gated membership)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood (RES-VWNN2T)
- [x] Scope defined (v1 = Tier A+B; C/D + MCP + strict-mode deferred)
- [x] Acceptance criteria documented (7 in ticket body)

**Scope:** IN — new `internal/aclaudit` analyzer (pure-policy Tier A + metamodel
Tier B), `rela acl audit` Kong command (`--json`, `--exit-code`), migrate the
two membership warns out of `Policy.Validate`. OUT — Tier C (graph), Tier D
(reachability), MCP `analyze_acl`, opt-in strict boot mode.

**Acceptance:** the 7 criteria in the ticket. Each maps to a test below.

## Research

- [x] `/research` run → RES-VWNN2T (done). Package boundary, finding taxonomy,
Validate-migration all decided there.
- [x] Existing solutions checked: `internal/validator` + `schema/validate_properties.go`
(store+metamodel cross-check pattern); `DBCmd`/`AnalyzeCmd` (Kong group +
`--exit-code`); `output.AnalysisResult` (JSON envelope).

**Research Doc:** RES-VWNN2T

## Approach

- [x] Technical approach chosen and documented
- [x] Builds on existing patterns
- [x] Alternatives considered (in research: aclaudit vs analysis vs acl)
- [x] Dependencies identified

**Technical approach (file by file):**

1. **`internal/aclaudit/aclaudit.go`** — core.
   - `type Severity int` (Critical/High/Medium/Low/Nit) + `String()`.
   - `type Finding struct { Rule string; Severity Severity; Subject, Detail, Fix string }`.
   - `type MetamodelReader interface` — narrow consumer-side: `HasEntityType(string) bool`,
`GetRelationDef(string) (RelationView, bool)` (or expose `.From []string`),
`EnumOptions(entityType, field string) ([]string, bool)`. Defined HERE (call
site), implemented by a thin adapter in the CLI over `*metamodel.Metamodel`.
   - `func Audit(p *acl.Policy, m MetamodelReader) []Finding` — runs all Tier A
checks (pure-policy, m may be nil-tolerant for A-only callers) then Tier B
(skipped if m == nil). Deterministic order (sort by severity then Rule then
Subject).
   - One small unexported func per check (A1..A10, B1..B7) appending Findings.
Reuse `acl` exported surface: `p.Roles`, `p.Assignments`, `p.RoleRelations`,
`p.MembershipRelation` + `acl.EveryoneRole`. NOTE: `membershipRelation()` is
unexported — either export an `EffectiveMembershipRelation()` on Policy (tiny
acl change) or replicate the trim/default here. Decide in impl; leaning export,
since the audit must see the SAME effective value the resolver walks.
2. **`internal/aclaudit/metamodel_adapter.go`** OR keep adapter in CLI — a
`metamodelReader` wrapping `*metamodel.Metamodel` implementing the interface via
`HasEntityType`, `GetRelationDef` (→ `.From`), and the enum lookup
(`EntityDef.Properties[f].Values` else `Metamodel.Types[prop.Type].Values`). Put
the adapter in the CLI package (consumer side) to keep `aclaudit` free of a
metamodel import IF we want aclaudit to depend only on acl. **Decision:**
aclaudit defines the interface; the adapter lives wherever the concrete
`*metamodel.Metamodel` is held = the CLI. That keeps aclaudit's arch-lint dep to
`[acl]` only for v1 (revisit when Tier C needs store). Update research note.
3. **`internal/cli/acl.go`** — `ACLCmd{ Audit AclAuditCmd }`; `AclAuditCmd{ JSON
bool; ExitCode bool }` with `Run(ctx, svc *cliServices) error`: load
`acl.LoadPolicy(filepath.Join(svc.Paths().Root, "acl.yaml"))` (handle
`errors.Is(os.ErrNotExist)` → "no acl.yaml; nothing to audit", exit 0); build
the metamodel adapter over `svc.Meta()`; `findings := aclaudit.Audit(p,
adapter)`; render text (`out.Write*`) or JSON (`output.AnalysisResult`); if
`--exit-code` and any Critical/High → return a non-zero-exit error.
4. **`internal/cli/kong.go`** — add `ACL ACLCmd \`cmd:"" help:"Audit the ACL policy (acl.yaml)."\``to the root`CLI` struct.
5. **`internal/acl/policy.go`** — remove `warnMembershipRelationHardening` + its
call in `Validate`; (likely) add exported `EffectiveMembershipRelation()`
wrapping `membershipRelation()` so the audit reads the same value.
6. **`internal/acl/policy_test.go`** — delete `TestPolicy_MembershipRelation_UngatedWarns`
/ `GatedNoWarn`; their behaviour re-asserted in `aclaudit` tests.
7. **`.go-arch-lint.yml`** — add `aclaudit: { in: internal/aclaudit }` and a deps
entry `aclaudit: mayDependOn: [acl]` (v1). cli already may depend on acl +
metamodel + aclaudit (verify cli deps list; add aclaudit).
8. **Docs** — `docs/security.md` / `GUIDE-acl-security.md`: note `rela acl audit`
as the hardening-check tool; regenerate.

**Alternatives rejected (research):** folding into `internal/analysis`;
extending `internal/acl` (arch-lint forbids acl→metamodel).

**Files to modify/create:**
- NEW `internal/aclaudit/{aclaudit.go, aclaudit_test.go}` (+ maybe metamodel adapter)
- NEW `internal/cli/acl.go` (+ `acl_test.go`)
- `internal/cli/kong.go`, `internal/acl/policy.go`, `internal/acl/policy_test.go`
- `.go-arch-lint.yml`
- `docs/security.md` + `docs-project/.../GUIDE-acl-security.md`

## Security Considerations

- [x] Input sources: `acl.yaml` (operator config) + metamodel (operator schema).
Read-only; the audit makes no writes and no auth decisions — it only reports.
- [x] The audit must not itself be a foot-gun: a clean policy MUST report zero
findings (no false criticals that train operators to ignore output). Each check
gated narrowly (e.g. A3 only fires when the everyone-role actually grants
write/permissions, not merely exists).
- [x] `--exit-code` semantics fixed: non-zero only on Critical/High, so Low/Nit
noise can't break a pipeline.
- [x] No sensitive info leak: findings name policy/schema identifiers the
operator already owns.

## Test Plan

- [x] Per-criterion scenarios; edge cases; negatives; integration

**Test scenarios (`aclaudit_test.go`, table-driven, a fakeMetamodelReader):**
- AC1 → un-gated membership + privileged assignment → A1 (high) present.
- AC2 → write-role on `everyone` → A3 (critical).
- AC3 → grant naming undeclared type → B1; `membership_relation` undeclared → B2.
- AC4 → clean well-gated policy → zero findings.
- Each remaining check (A1b/A2/A4/A5/A6/A7/A9/A10, B3/B4/B5/B7) → one positive +
one negative case proving it doesn't false-fire on clean config.
- Determinism: findings sorted stable (severity, rule, subject).
- Migration: the two ex-Validate behaviours (un-gated/inert membership) now
assert via `aclaudit` (relocated from policy_test.go).

**CLI tests (`acl_test.go`):** golden text + `--json` envelope shape;
`--exit-code` returns non-zero on a critical-bearing policy, zero on clean;
missing acl.yaml → exit 0 with a "nothing to audit" message.

**Edge cases:** nil metamodel reader (Tier A still runs, Tier B skipped);
wildcard `["*"]` interaction with A9 vs B1 (a `"*"` grant is not an undeclared
type — B1 must skip `"*"`); `everyone` role with only `read` (A3 should NOT fire
— read isn't privileged escalation, only write/permissions).

## Risk Assessment

- [x] Risks + mitigations; effort

**Risks:**
- MED: false-positive criticals erode trust. Mitigation: AC4 clean-policy test +
a negative case per check; conservative gating (A3 = write/perms only).
- LOW: `membershipRelation()` is unexported — audit could drift from the real
effective value. Mitigation: export `EffectiveMembershipRelation()` and have
both resolver and audit use it (single source of truth, extends TKT-Z8A62F's
accessor discipline).
- LOW: arch-lint/plimsoll. Mitigation: narrow interface; new component entry;
run `just arch-lint` + `just plimsoll` before PR.

**Effort:** m (~250 LOC core + checks, ~300 LOC tests, CLI + arch-lint + docs).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created at implementation

**Documentation Impact:**
- [x] docs/security.md + GUIDE-acl-security.md — document `rela acl audit`
- [x] docs/cli-reference.md (if generated) — the new command
- [ ] ~~metamodel.md / data-entry.md~~ (N/A)

## Design Review

- [x] Ran `/design-review` before implementation
- [x] All significant findings folded into the gating rules below

**Design Review Findings (all addressed):**

- **RR-LXI3NW (significant)** — "privileged role" is undefined; the ACL has no
built-in privilege notion. RESOLUTION: define `isPrivileged(role)` in `aclaudit`
= grants any write (`Create`/`Update`/`Delete` non-empty, incl. `"*"`) OR holds
any `Permissions`. A2/A3 reference this one helper; the highest-signal sub-case
(a `delegate-*` permission or a wildcard write) drives wording. Documented +
unit-tested.
- **RR-UR0LJU (significant)** — A9 would false-positive on the documented
`everyone: read: ["*"]`. RESOLUTION: A9 flags ONLY write/permission wildcards
(`create/update/delete: ["*"]` or a delegate permission); `read: ["*"]` is NEVER
flagged. Mandatory negative test: `everyone: read: ["*"]` → zero A9.
- **RR-O7H3GY (minor)** — A6 can be an intentional lockdown. RESOLUTION: A6 →
**low/info**, worded as a question ("no principal can write X — intended?").
- **RR-TZ2S3G (minor)** — Tier-B grant-list checks must skip the `"*"` sentinel.
RESOLUTION: B1 (and any check reading verb lists) skips `"*"`; affordance map
KEYS (no wildcard) are checked directly. Mandatory test: wildcard role → zero
B1.

**New audit-wide invariant (from these findings):** every check ships with a
NEGATIVE test proving silence on clean/legitimate config, AND the full
docs/acl-overview.md worked-example policy is run through `Audit` as a golden
"expected findings" fixture — so a future check can't silently start flagging
the recommended baseline.
