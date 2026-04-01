---
id: PLAN-ULY5
type: planning-checklist
title: 'Planning: CI validation for checklist completion'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**

- Update the `rela-tickets` CI job to use `rela validate --check all` instead of individual
  `rela analyze` commands
- The `validate --check` command exits with error code 1 on failures, while `analyze` commands
  only print warnings and return success

**Out of scope:**

- No changes to the `rela validate` command itself (already implemented in TKT-9D7P)
- No changes to validation rules (already implemented in TKT-Y2JW)

**Acceptance Criteria:**

1. CI fails when validation errors exist (e.g., incomplete checklists on done tickets)
2. CI passes when all validations succeed
3. CI output shows clear error messages for failures

## Research

- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: CI workflow change only)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal CI change)
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- The `rela validate --check` command was implemented in TKT-9D7P
- Current CI uses `rela analyze cardinality/orphans/properties/validations` which return 0 even on
  warnings
- The validate command with `--check all` runs cardinality, properties, and validations with proper
  exit codes

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Replace the four `rela analyze` commands in CI with a single `rela validate --check all` command.
This simplifies CI and ensures proper failure on validation errors.

Current:

```yaml
../bin/rela analyze cardinality
../bin/rela analyze orphans
../bin/rela analyze properties
../bin/rela analyze validations
```

New:

```yaml
../bin/rela validate --check all
```

Note: orphans check is not included in validate --check because orphan entities are informational,
not errors.

**Alternatives considered:**

1. Add `--strict` flag to analyze commands to fail on warnings - rejected because validate already
   provides this functionality
2. Keep analyze commands but add `|| exit 1` - rejected because it's hacky and validate is the
   proper solution

**Files to modify:**

- `.github/workflows/ci.yml`

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: CI workflow change only)
- [x] ~~Input validation approach defined~~ (N/A: no user input)
- [x] ~~Security-sensitive operations identified~~ (N/A: no security operations)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: CI output is expected to be visible)

**Input Sources & Validation:**

N/A - This is a CI workflow change that runs the existing `rela validate` command.

**Security-Sensitive Operations:**

N/A - No security-sensitive operations.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: manual verification via CI run)

**Test Scenarios:**

| Acceptance Criterion | Test Scenario |
|---------------------|---------------|
| CI fails on validation errors | Create PR with incomplete checklist, verify CI fails |
| CI passes when validations succeed | Create PR with complete checklists, verify CI passes |
| Clear error messages | Check CI output shows specific validation failures |

**Edge Cases:**

- PR with only doc changes (no ticket entities) - should still run validation on existing entities

**Negative Tests:**

- Incomplete checklist on done ticket should fail CI
- Missing required relations should fail CI

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed~~ (N/A: no security concerns)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Existing PRs may fail | Medium | Low | Document the change, fix any blocking issues first |
| validate command has different behavior | Low | Low | Command is already tested in TKT-9D7P |

**Effort:** xs (single file change)

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal CI change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, not enhancement)

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: trivial change)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review needed)

**Design Review Findings:** N/A - This is a trivial CI configuration change.
