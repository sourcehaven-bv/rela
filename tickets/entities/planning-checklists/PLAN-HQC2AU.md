---
id: PLAN-HQC2AU
type: planning-checklist
title: 'Planning: Per-face fields: policy'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `type@face` keys in a role's `fields:` and `options:` blocks (the
write-side affordances). Load-time validation of the key. Face-aware A14
(everyone opens a block) and aclaudit B4/B5 field checks. Docs. Out: `type@face`
keys in `visible:` and `relations:`. They stay a load error, now with a message
that says face keys are only accepted in `fields:` and `options:`. Per-face
`visible:` needs the search lowering (`SearchFieldScope`, query-scope
visibility, historical closed world) to become face-aware; follow-up ticket.
`entity.face` in `when:` is not added.

**Acceptance Criteria:**
1. `fields: {document@vastgesteld: [ondertekend]}` on a role with
`update: [document@concept, document@vastgesteld]`: on a vastgesteld row only
`ondertekend` is writable; on a concept row every field is writable.
2. A role with both `document: [titel]` and `document@vastgesteld: [ondertekend]`
writes `titel` on concept rows and `titel` + `ondertekend` on vastgesteld rows
(union within the role, as across roles).
3. Same for `options:`.
4. Load errors: undeclared face, faceless type with a face, empty face, two `@`,
unknown type; `visible:`/`relations:` face key refused with a clear message.
5. A14: everyone opening a face block is reported only for that face.
6. aclaudit B4/B5 check the fields of a face key against its type.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one option already matches the existing face grant syntax)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal policy semantics)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal policy semantics)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `read:`/`update:` face grants
(`internal/acl/worldgrant.go` parseStateGrant, `rolegrant.go` GrantsVerbOnFace);
`grantKeyApplies` in `internal/acl/traversal.go` already treats `type@face` keys
as naming the type; aclaudit `grantEntityType`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `affordances.grantKey` gains a `face` part; a bare key
compiles under face "". `compileRole` splits `fields:`/`options:` keys and
validates the face against the type's declared faces. `applyRoleBlocks` folds
the bare block and the block for the row's face; the role counts as declaring
the dimension when either exists. `when:` of a face block compiles against the
base type's env. Alternative rejected: `entity.face` in `when:` (would also need
lowering for `visible:`, and differs from the grant syntax). Alternative
rejected: most specific key wins (a face block replacing the type block); it
subtracts within a role, while every other combination rule is a union.

**Files to modify:** internal/affordances/resolver.go, validate.go;
internal/acl/everyoneblocks.go; internal/aclaudit/tier_b.go;
docs/acl-overview.md, docs/acl-security.md; tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Operator-authored acl.yaml. The face must be a
declared face of the type (allowlist); anything else fails the load.

**Security-Sensitive Operations:** Write field gate. A face block only grants
within a role that already holds the row verb; the row ACL is unchanged.
Refusing face keys in `visible:` keeps redaction and the search scope unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Table tests in internal/affordances for criteria 1-3
(FieldVerdicts and CreateFieldVerdicts); load-error table for criterion 4; acl
test for A14; aclaudit test for B4.

**Edge Cases:** Face key on a faceless type; face key while the role has no
update on that face (the block grants nothing, as today); create of a faced row.

**Negative Tests:** Load errors listed in criterion 4.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A consumer that reads `role.Fields` keys as type names misreads a
face key. Mitigation: all consumers were enumerated (affordances,
everyoneblocks, aclaudit) and are updated. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/acl-overview.md (scoping a grant to a content
state), docs/acl-security.md.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Self-review. Significant: A14 and B4 read keys as
types (addressed: in scope). Significant: per-face `visible:` would need the
search lowering (addressed: out of scope, explicit load error, follow-up
ticket).
