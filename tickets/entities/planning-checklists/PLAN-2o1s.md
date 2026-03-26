---
id: PLAN-2o1s
status: done
title: 'Planning: Allow configuration of short ID capitalization'
type: planning-checklist
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
- IN SCOPE: Add `id_caps` field to metamodel entity definitions to control suffix capitalization
- IN SCOPE: Support values: `upper` (default), `lower`
- IN SCOPE: Change default from lowercase suffix to uppercase suffix
- IN SCOPE: Prefix always stays as defined in metamodel (not affected by id_caps)
- OUT OF SCOPE: Migration for existing IDs (they remain as-is)

**Acceptance Criteria:**
1. `id_caps: upper` produces `TKT-A1B2` (prefix as-is, suffix uppercase) - NEW DEFAULT
2. `id_caps: lower` produces `TKT-a1b2` (prefix as-is, suffix lowercase - current behavior)
3. Omitting `id_caps` defaults to `upper`
4. Invalid values are rejected at metamodel load time

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- Current logic in `internal/model/id.go:219` uses `strings.ToUpper(prefix) + string(b)` where `b` uses lowercase base36 chars
- Need to: (1) stop uppercasing prefix, (2) add option to uppercase suffix
- `EntityDef` struct pattern for optional fields like `id_type`, `id_prefix`

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add `IDCaps` field to `EntityDef` struct in `internal/metamodel/types.go`
2. Add `IDCapsUpper`, `IDCapsLower` constants
3. Add `GetIDCaps()` helper method in `internal/metamodel/entity_def.go`
4. Modify `GenerateShortID()` to accept capitalization option
5. Update `generateRandomBase36()`: remove `strings.ToUpper(prefix)`, add suffix case option
6. Update `Workspace.GenerateID()` to pass capitalization to model
7. Add validation in metamodel loader

**Files to modify:**
- `internal/metamodel/types.go` - Add IDCaps field and constants
- `internal/metamodel/entity_def.go` - Add GetIDCaps() helper
- `internal/metamodel/loader.go` - Add validation for id_caps values
- `internal/model/id.go` - Modify GenerateShortID and generateRandomBase36
- `internal/workspace/workspace.go` - Pass capitalization to model

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Source: `id_caps` field in metamodel.yaml
- Validation: Allowlist (`upper`, `lower`)
- Invalid input: Error at metamodel load time

**Security-Sensitive Operations:**
- None - capitalization doesn't affect security

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. Unit test `generateRandomBase36()` with upper/lower modes
2. Unit test `GetIDCaps()` default behavior
3. Metamodel validation test for invalid `id_caps` values

**Edge Cases:**
- Empty `id_caps` field → defaults to `upper`
- Prefix without hyphen → hyphen added, prefix case preserved

**Negative Tests:**
- `id_caps: mixed` → should error at metamodel load

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Breaking change: New default (uppercase suffix) differs from old (lowercase suffix)
  - Mitigation: Users can use `id_caps: lower` for old behavior
- Existing tests may assume lowercase suffix
  - Mitigation: Update fuzz tests

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: simple config change)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** N/A - straightforward config addition
