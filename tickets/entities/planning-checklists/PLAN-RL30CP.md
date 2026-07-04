---
id: PLAN-RL30CP
type: planning-checklist
title: 'Planning: ACL: make the membership relation (member-of) configurable via membership_relation: in acl.yaml'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: Make the relation the ACL resolver walks for group membership configurable
via a new `membership_relation:` key in `acl.yaml`. Default `"member-of"`, fully
backwards-compatible. Add hardening validation warnings. Update docstrings +
operator docs.

OUT: Multiple membership relations at once (single field, not a list).
`_actions` UI changes. Migration tooling. Authentication. Any change to read
filtering, inheritance, or the delegate-X gate logic itself. A dedicated
authz-misconfiguration validator → follow-up TKT-TS0J5K.

**Acceptance Criteria:** (see Test Plan for the concrete test for each)
1. `membership_relation: heeft_rol` + `assignments: {engineering: editor}` +
edge `alice --heeft_rol--> engineering` → alice gets `editor` with `Source{Kind:
SourceGroup, Group: "engineering"}`.
2. Unset `membership_relation:` → default `member-of`; existing edges walked.
3. `membership_relation: heeft_rol` + only a `member-of` edge → no role.
4. Transitive: A --heeft_rol--> B --heeft_rol--> C, `assignments: {C: editor}`
→ A gets editor via group C.
5. `Policy.Validate`: `membership_relation: heeft_rol` with no
`role_relations.heeft_rol` → hardening warning fires (Validate returns nil).
6. Blank/whitespace `membership_relation:` → default member-of (NOT match-all).

## Research

- [x] ~~Run /research~~ (N/A: small, well-specified single-field change)
- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `InheritRolesThrough []string` (policy.go:57) is the closest prior art: a
policy-configured relation-name list the resolver walks (resolver.go:194).
- ONE production literal: `resolver.go:65`.
- Warning style: `slog.Warn("acl: ...", k, v)` per policy.go:260, storegraph.go:44.
- Group-source assertion model: features_test.go:41.
- **Key risk found in design review:** `StoreGraph.OutgoingRelations` passes the
relation name as `store.RelationQuery.Type`, and `Type==""` means "all relation
types" (CLAUDE.md; the existing Validate guard rejects blank
InheritRolesThrough/RoleRelations keys for exactly this reason). So a blank
membership relation reaching the resolver = walk-all-edges over-grant.
- `NewDeclarative` does NOT call `Validate` (declarative.go:55-66); 9+ test
sites build `&Policy{...}` literals directly. Defaulting only in Validate is
therefore insufficient and unsafe.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach (revised after design review — RR-LFMR7S/XKQZ1N/WRFCW5):**

1. `policy.go`: add `MembershipRelation string yaml:"membership_relation"` to
`Policy` (after `UserEntityType`); add `"membership_relation": true` to
`knownPolicyKeys`; add const `defaultMembershipRelation = "member-of"`.
2. **Non-mutating effective-name accessor** (the correctness mechanism):
`func (p *Policy) membershipRelation() string` returns
`defaultMembershipRelation` when `isBlank(p.MembershipRelation)`, else the
configured value. This handles "" AND whitespace-only (reuses existing
`isBlank`). The resolver reads through THIS, never the raw field — so every path
(LoadPolicy, NewDeclarative, direct `&Policy{}` test literals) is safe
regardless of whether Validate ran. Do NOT rely on a Validate mutation for
correctness (respects the Policy()-is-immutable contract, declarative.go:75).
3. `resolver.go:65`: `OutgoingRelations(ctx, n, r.d.policy.membershipRelation())`.
Update `computeGlobals` (8-13) and `walkMembers` (49-53) docstrings to "the
configured membership relation (default member-of)".
4. `policy.go` `Validate()`: add two advisory `slog.Warn` calls (non-fatal,
gated on the EFFECTIVE name != default so the default path is silent):
   - effective membership relation != default but `Assignments` empty.
   - effective membership relation != default but no
`role_relations.<rel>.requires_permission` → escalation foot-gun, points to
docs/security.md. Validate does NOT mutate the field (decision: keep the
accessor as the single source of truth; no value in also writing it back).
5. Docstrings: `Policy` struct godoc (new bullet); `RoleRelationDef` godoc
(rename "Escalation risk for the `member-of` relation" → "for the configured
membership relation (default `member-of`)").
6. Docs: `docs/acl-overview.md`, `docs/security.md` §"Hardening member-of",
`docs/concepts.md`.

**Alternatives rejected:** (a) default only in Validate — unsafe, see research.
(b) mutate the receiver in Validate — fragile, violates immutability contract,
and still leaves the NewDeclarative-direct path unprotected. (c) `[]string` list
— out of scope. (d) default in NewDeclarative — would miss the Validate-only /
print paths and still leaves the raw field blank; the accessor is the one place
every reader funnels through.

**Files to modify:**
- `internal/acl/policy.go` (field, const, accessor, knownPolicyKeys, 2 warnings,
2 docstrings)
- `internal/acl/resolver.go` (line 65 + 2 docstrings)
- `internal/acl/resolver_test.go` (new resolver cases)
- `internal/acl/policy_test.go` (accessor + warning tests)
- `docs/acl-overview.md`, `docs/security.md`, `docs/concepts.md`

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined (allowlist-style: blank/whitespace → safe default)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `membership_relation:` is operator-controlled
acl.yaml config. The dangerous value is blank/whitespace (→ match-all walk =
over-grant); the accessor collapses all blank forms to the safe default, which
is the allowlist-style mitigation. No string-injection surface beyond that.

**Security-Sensitive Operations:** membership relation drives role attribution.
Foot-gun (operator points at a writable domain relation → self-promotion) is
surfaced via the new hardening warning + docs, mirroring existing member-of
guidance. Delegate-X gate unchanged; already applies to whatever relation is in
`role_relations`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**
1. AC1 — resolver_test: `MembershipRelation:"heeft_rol"`, edge alice→engineering,
assignment engineering=editor → role `editor` with `Source{Kind:SourceGroup,
Group:"engineering"}`.
2. AC2 — resolver_test: empty `MembershipRelation`, member-of edge → role granted
(default path).
3. AC3 — resolver_test: `MembershipRelation:"heeft_rol"`, only a member-of edge →
NO role (negative: wrong relation not followed). Also assert via the fake
graph's `outgoingByRel` that the walk queried `heeft_rol`, NOT `member-of` and
NOT "" — pins that we don't fall into match-all.
4. AC4 — resolver_test: transitive heeft_rol A→B→C, assignment C=editor → A editor.
5. AC5 — policy_test: `Validate()` with `MembershipRelation:"heeft_rol"` and no
`role_relations.heeft_rol` → capture slog via a test handler, assert the
hardening warning fired and Validate returned nil.
6. AC6 — policy_test: table over {"", "   ", "\t"} → `membershipRelation()` ==
"member-of"; and "heeft_rol" → "heeft_rol".

**Edge Cases:** explicit `membership_relation: ""`/whitespace → default (AC6).
`membership_relation: member-of` explicit = default, no warning.
Cycle/self-loop/ depth-cap under custom name = same as member-of (mechanism is
name-agnostic; AC4 covers transitivity).

**Negative Tests:** AC3 (wrong name → no membership, and not match-all). AC6
ensures blank never reaches the store as a match-all query.

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- MED→mitigated: blank relation = match-all over-grant. Mitigation: the
`membershipRelation()` accessor (every read funnels through it) + AC3/AC6 tests.
- LOW: stray `"member-of"` literal drift. Mitigation: shared const + grep.
- LOW: warning noise on default. Mitigation: gate on effective != default.

**Effort:** s (~35 LOC prod incl. accessor, ~170 LOC tests, ~30 lines docs).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/acl-overview.md — `membership_relation:` optional + default
- [x] docs/security.md — generalise "Hardening member-of"
- [x] docs/concepts.md — "(default member-of, configurable)"
- [x] Godoc on `Policy` + `RoleRelationDef`
- [ ] ~~docs/metamodel.md / cli-reference.md / data-entry.md~~ (N/A)

## Design Review

- [x] Ran `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-LFMR7S (significant — default-in-Validate
insufficient; use effective-name accessor), RR-XKQZ1N (minor — don't mutate
immutable Policy; accessor instead), RR-WRFCW5 (minor — whitespace → default).
All three folded into the revised Approach above. No critical findings.
