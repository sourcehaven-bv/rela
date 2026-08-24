---
id: PLAN-QI9SFC
type: planning-checklist
title: 'Planning: Gate the membership relation against ACL self-promotion'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope (ticket items 1–3):

1. Extract the A1-ungated-membership predicate from `aclaudit/tier_a.go`
into `internal/acl` as `Policy.MembershipSelfPromotionOpen()` (plus
`RoleDef.IsPrivileged()`); aclaudit delegates to it. Behaviour identical.
2. rela-server logs a prominent startup warning (`warnUngatedMembership` in
`internal/appbuild`) when a loaded policy has privileged assignments and an
ungated membership relation. Warning, not refusal.
3. Docs: `docs/acl-security.md` + `docs-project/entities/guides/GUIDE-acl-security.md`
record the warning and the coming deployment requirement (world grants + ungated
membership = load refusal).

OUT of scope (explicitly deferred to TKT-DN37J2):

4. The `Policy.Validate` hard load refusal when a non-default-world read
grant coexists with ungated membership. No latent always-false
`hasNonDefaultWorldGrant` hook is built here — it would be untestable dead code
pre-committing Step 3's data model. Also out of scope: an unconditional refusal
(would break existing trusted-team deployments).

**Acceptance Criteria:**

1. `Policy.MembershipSelfPromotionOpen()` returns true iff at least one
assignment targets a declared privileged role AND the effective membership
relation has no `requires_permission` gate — pinned by the table test
`TestPolicy_MembershipSelfPromotionOpen` (gated/ungated, privileged vs read-only
role, undeclared role, configured relation name, whitespace trimming).
2. aclaudit A1 behaviour is unchanged: existing A1 tests in
`internal/aclaudit` still pass against the delegating implementation;
`isPrivileged` delegates to `RoleDef.IsPrivileged` so A2/A3 share the same
definition.
3. Startup warning fires exactly when the predicate is true —
`TestBuildACL_UngatedMembership_WarnsAtStartup` (fires, names relation + fix +
docs) and `TestBuildACL_MembershipWarning_QuietWhenSafe` /
`TestBuildACL_NoPolicy_NoMembershipWarning` (quiet for gated policy, read-only
assignment, no assignments, no acl.yaml).
4. Docs describe the warning and the future refusal condition in both the
external doc and the docs-project guide.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach fixed by `.ignored/pointer-design.md` §12.1 and its review pass; alternatives — refusal vs warning, predicate placement — were evaluated there and in the ticket text)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal refactor + logging, no library surface)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: project-internal predicate extraction)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (small change; design fixed by FEAT-9CD2MX design doc
§12.1; the feature-level research lives in RES-NH3P12)

**Existing Solutions:**

- The predicate already existed inside `aclaudit/tier_a.go`
(`assignsAnyPrivilegedRole` + `requiresPermissionFor`); this ticket moves it to
`internal/acl` beside `Policy.EffectiveMembershipRelation()`, the accessor it
must agree with.
- Privilege definition (`isPrivileged`) shared with A2/A3 per
RR-LXI3NW/RR-UR0LJU/RR-EG5D3E — read grants are not privilege.
- Startup-warning placement follows the existing pattern of boot-time
`slog.Warn` diagnostics in `appbuild.buildACL` (policy is validated against the
metamodel there; the warning sits directly after).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Move the A1 condition onto `acl.Policy` as the single implementation
(`MembershipSelfPromotionOpen`), export `RoleDef.IsPrivileged`, have aclaudit
delegate (import direction aclaudit→acl already exists). Call the same predicate
from `appbuild.buildACL` after metamodel validation and log a `slog.Warn` naming
the relation, the one-line fix, the docs page, and `rela acl audit`.
Warning-not-refusal is a recorded decision: unconditional refusal breaks
existing trusted-team deployments; the refusal is scoped to the world-grant
condition and lands with TKT-DN37J2.

**Files to modify:**

- `internal/acl/policy.go` (+ new `internal/acl/membership_gate_test.go`)
- `internal/aclaudit/aclaudit.go`, `internal/aclaudit/tier_a.go`
- `internal/appbuild/appbuild.go` (+ new `internal/appbuild/appbuild_membership_warn_test.go`)
- `docs/acl-security.md`, `docs-project/entities/guides/GUIDE-acl-security.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `acl.yaml` (operator-authored policy) is the only input; it is already
parsed/validated by existing policy loading. The predicate only reads parsed
fields (`Assignments`, `Roles`, `RoleRelations`, `MembershipRelation` via the
trimming accessor) — no new parsing surface.

**Security-Sensitive Operations:**

- The change is itself a security control (escalation-path detection). Key
property: audit finding (A1) and boot warning evaluate the SAME predicate, so
advisory and enforcement views cannot drift. False-negative risk (predicate too
narrow) is bounded by the shared A2-symmetric privilege definition and pinned by
tests; false-positive risk (warning noise) is bounded by the read-only-role and
undeclared-role exclusions.
- The warning logs only the relation name and remediation — no policy
contents, principals, or secrets.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 → `internal/acl/membership_gate_test.go`:
`TestPolicy_MembershipSelfPromotionOpen` (10 table cases) +
`TestRoleDef_IsPrivileged` (8 cases incl. wildcard and permissions-only).
- AC2 → existing `internal/aclaudit` A1 tests (fires at High severity;
quiet when gated / read-only) run against the delegating code.
- AC3 → `internal/appbuild/appbuild_membership_warn_test.go`: full
`appbuild.New` boot on a temp project (integration-level), capturing slog
output; warning fires for the open policy, quiet for the three safe shapes and
for a project without acl.yaml.

**Edge Cases:**

- Configured (non-default) membership relation: gate must be read on the
EFFECTIVE relation, not the default `member-of` (pinned both directions).
- Whitespace-padded `membership_relation` value (trimming accessor).
- Assignment naming an undeclared role (confers nothing → not open; A4
reports it separately).
- Permission-only role (no write verbs) still counts as privileged.
- Mixed assignments: one privileged among read-only ones is enough.

**Negative Tests:**

- Gated policy, read-only-only assignments, no assignments, no acl.yaml →
predicate false / no warning (the quiet cases, so the warning does not become
noise).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Behaviour drift during extraction → mitigated: aclaudit tests unchanged
and passing; delegation keeps call sites reading the same.
- Warning fatigue for trusted-team deployments → accepted and deliberate:
single line at boot, suppressible by actually gating the relation (one-line fix
stated in the warning).
- Deferred refusal (item 4) forgotten → recorded as acceptance criterion on
TKT-DN37J2 and documented in both docs files as a coming requirement.

Effort: m (matches ticket property).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/acl-security.md — startup warning + coming refusal requirement (done in this change)
- [x] docs-project GUIDE-acl-security — same content, guide form (done in this change)
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel features)
- [x] ~~docs/cli-reference.md~~ (N/A: no new commands; `rela acl audit` unchanged)
- [x] ~~docs/data-entry.md~~ (N/A: no UI changes)
- [x] ~~CLAUDE.md~~ (N/A: no new patterns)
- [x] ~~README.md~~ (N/A: no project-level changes)

## Design Review

- [x] Run `/design-review` before starting implementation (run retroactively — implementation preceded the paperwork; reviewed the design as embodied)
- [x] All critical/significant findings addressed in plan (none found; two minor findings documented)

**Design Review Findings:**

- RR-62ZH2M (minor, wont-fix): warning fires on all appbuild consumers
incl. CLI, not only rela-server — deliberate, single shared call site.
- RR-S7A16Q (minor, deferred): chained escalation via an ungated
non-membership role-relation (A2 domain) is boot-warning-blind; audit A2 covers
it. Flagged to the architect for TKT-DN37J2, whose refusal keys on the same
predicate and would inherit the blind spot.
- Nit (no entity): injected-ACL callers (read-only server, MCP NopACL) skip
the warning — correct, no write surface / no policy there.
