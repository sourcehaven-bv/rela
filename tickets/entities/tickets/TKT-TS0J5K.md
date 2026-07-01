---
id: TKT-TS0J5K
type: ticket
title: 'ACL: dedicated authorization-misconfiguration validator / audit insights (escalation foot-guns, dead assignments, un-gated membership)'
kind: enhancement
priority: medium
effort: m
status: review
---

Follow-up from TKT-Z8A62F. Design settled in RES-VWNN2T; design-review in
RR-LXI3NW / RR-UR0LJU / RR-O7H3GY / RR-TZ2S3G.

## What

Build `rela acl audit` — an **on-demand** linter for `acl.yaml`, separate from
the boot-time `Policy.Validate` gate. Reports escalation foot-guns, dead/inert
config, and policy-vs-metamodel drift as **severity-ranked findings**. Advisory
by default (never blocks boot); CI gating opt-in via `--exit-code`.

## Architecture (RES-VWNN2T + design-review)

- **New `internal/aclaudit` package.** arch-lint forbids `acl → metamodel`, so the
audit (needs Policy + schema) cannot live in `internal/acl`. `aclaudit` defines
a narrow consumer-side `MetamodelReader` interface; the concrete adapter over
`*metamodel.Metamodel` lives in the CLI → `aclaudit` arch-lint dep = `[acl]`
only for v1. (Required anyway: `Metamodel`/`EntityDef` are over the plimsoll
line.)
- **Typed `Finding{Rule, Severity, Subject, Detail, Fix}` + 5-level severity**
(Critical/High/Medium/Low/Nit). `--exit-code` gates on Critical/High.
- **`isPrivileged(role)`** (RR-LXI3NW) = grants any write (Create/Update/Delete
non-empty incl `"*"`) OR holds any Permissions. A2/A3 reference it.
- **CLI:** Kong `ACLCmd{ Audit }` → `rela acl audit`, `--json`
(`output.AnalysisResult` envelope) + `--exit-code`.

## Validate migration (behaviour change)

Move the two membership `slog.Warn`s out of `Policy.Validate` into `aclaudit`
(A1/A1b); `Validate` returns to a pure structural gate. Boot no longer logs
them. Relocate `TestPolicy_MembershipRelation_UngatedWarns`/`GatedNoWarn` to the
audit suite. Add exported `Policy.EffectiveMembershipRelation()` so audit +
resolver share one source of truth.

## v1 check set (gating per design-review)

**Tier A (pure-policy):** A1 un-gated membership relation confers an assigned
role (incl. default member-of) — **high**; A1b non-default membership +empty
assignments — **low**; A2 un-gated role-relation conferring a privileged role —
**high**; A3 privileged role on `everyone` — **critical**; A4 assignment→
undeclared role — **medium**; A5 confers→undeclared role — **medium**; A6
requires_permission no role grants (phrased as a question, lockdown may be
intended) — **low**; A7 dead permission — **low**; A9 WRITE/permission wildcard
on a non-everyone role (NEVER read:["*"]) — **medium**; A10 whitespace/case in a
name field — **low**.

**Tier B (metamodel, skip `"*"` sentinel):** B1 grant/affordance-key names an
undeclared entity type — **high**; B2 membership/role/inherit relation
undeclared — **high**; B3 membership relation `from` ∌ user_entity_type (via
ValidateRelation) — **medium**; B4 fields/visible grant names an undeclared
field — **medium**; B5 options grant outside the field's enum — **medium**; B7
user_entity_type undeclared — **high**.

**Audit-wide invariant:** every check ships a negative test; the full
docs/acl-overview.md worked-example policy is a golden zero/expected-findings
fixture (so no future check flags the recommended baseline).

## Out of scope (follow-ups)

Tier C (graph: membership relation has zero edges; privileged group reachable by
everyone) — needs store; Tier D (reachability model-check); MCP `analyze_acl`;
opt-in strict boot mode.

## Acceptance

1. Un-gated `member-of` + privileged assignment → A1 (high); `--exit-code`
non-zero.
2. Write-role on `everyone` → A3 (critical).
3. Grant naming undeclared type → B1; `membership_relation` undeclared → B2.
4. Clean well-gated policy → zero findings, exit 0 (incl. the docs example with
`everyone: read: ["*"]`).
5. `--json` emits via `output.AnalysisResult`.
6. Membership warns gone from `Validate`; reproduced as aclaudit findings; tests
relocated; `Validate` still hard-errors on structural invariants.
7. `just arch-lint` + `just plimsoll` pass.
