---
id: PLAN-XWWN
type: planning-checklist
title: 'Planning: CLI validation command for CI integration'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN SCOPE:
- Add `--check` flag to `rela validate` command to run entity/relation validations
- Support check types: `cardinality`, `properties`, `validations`, `all`
- Support filtering validations by rule name: `--check validations:rule-name`
- Support filtering validations by entity type: `--check validations:@entity-type`
- Return exit code 1 when validation errors are found
- Support existing `-o json` and `-q` flags
- Multiple `--check` flags can be combined

OUT OF SCOPE:
- New validation types (use existing analyze logic)
- Changes to analyze command behavior
- View-scoped validation (covered by FEAT-drlm)
- Orphan/duplicate/gap checks (these are informational, not validation errors)
- Glob/pattern matching for rule names (keep it simple with exact match)

**Acceptance Criteria:**

1. `rela validate --check cardinality` runs cardinality analysis, exits 1 on violations
   - Test: Create entity missing required relation, verify exit code 1
2. `rela validate --check properties` runs property validation, exits 1 on errors
   - Test: Create entity with invalid enum value, verify exit code 1
3. `rela validate --check validations` runs all custom validations, exits 1 on errors (not warnings)
   - Test: Create entity that violates a severity=error rule, verify exit code 1
4. `rela validate --check validations:rule-name` runs only the named validation rule
   - Test: Run with specific rule name, verify only that rule runs
5. `rela validate --check validations:@ticket` runs only validations for ticket entity type
   - Test: Run with entity type filter, verify only matching rules run
6. `rela validate --check all` runs all three check categories
   - Test: Run with mixed issues, verify all are reported
7. Multiple checks: `rela validate --check cardinality --check validations:rule1 --check validations:rule2`
   - Test: Verify all specified checks run and aggregate exit code
8. Default behavior (no --check flag) unchanged - only validates config files
   - Test: Verify existing behavior still works
9. JSON output with `-o json` includes validation results
   - Test: Parse JSON output, verify structure matches AnalysisResult
10. Quiet mode `-q` suppresses non-error output
    - Test: Verify only errors printed with -q

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

No external libraries needed. The codebase already has all required components:

- `workspace.CheckCardinality(opts)` - `internal/workspace/analysis.go:158-169`
- `workspace.ValidateProperties(opts)` - `internal/workspace/analysis.go:307-323`
- `workspace.RunValidations(opts)` - `internal/workspace/analysis.go:331-334`
- `writeAnalysisJSON()` helper - `internal/cli/analyze.go:54-71`
- `output.AnalysisResult` struct - `internal/output/output.go:482-488`
- `errors.ExitError` for testable exit codes - `internal/errors/errors.go:67-81`

The analyze commands already implement these checks but don't set exit codes. The validate command currently only checks config files but can be extended.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Add `--check` flag** to validate command (string slice, repeatable)
   - Valid values: `cardinality`, `properties`, `validations`, `all`
   - Validation filtering: `validations:rule-name` or `validations:@entity-type`
   - Default: empty (existing config-only behavior)

2. **Extend validate command logic**:
   - If `--check` flags present, initialize workspace (call `workspace.DiscoverAndNew`)
   - Run requested checks by delegating to existing workspace methods
   - Aggregate results and set exit code based on errors found

3. **Use ExitError pattern** instead of direct `os.Exit(1)`:
   - Return `errors.NewExitError(1)` for testability
   - Root command already handles ExitError in `Execute()` at `cli/root.go:86-88`

4. **Aggregate results structure**:
   ```go
   type ValidationCheckResult struct {
       Cardinality []workspace.CardinalityViolation
       Properties  []workspace.EntityPropertyErrors
       Validations []workspace.ValidationViolation
   }
   ```

5. **Exit code logic**:
   - Exit 0: No errors found
   - Exit 1: Any cardinality violations, property errors, or validation errors (severity=error)
   - Warnings don't cause non-zero exit

**Alternatives Considered:**

A. **Add flags to analyze command instead**: Rejected because `analyze` is for exploration/reporting, `validate` is for CI gates. Semantic separation is cleaner.

B. **Create new `rela check` command**: Rejected because `validate` already exists for validation purposes and this extends it naturally.

C. **Use analyze exit codes**: Rejected because changing analyze behavior could break existing scripts.

**Files to modify:**

1. `internal/cli/validate.go` - Add --check flag, extend RunE logic
2. `internal/cli/validate_test.go` - Add tests for new functionality (new file)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `--check` flag values: Allowlist validation against known check types
- Invalid check type: Return clear error message, exit 1

**Security-Sensitive Operations:**

- File system access: Uses existing workspace methods, no new file access patterns
- No network operations, authentication, or cryptographic operations

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test | Expected |
|----|------|----------|
| 1 | Run `--check cardinality` with entity missing required relation | Exit 1, violation listed |
| 2 | Run `--check properties` with invalid enum value | Exit 1, error listed |
| 3 | Run `--check validations` with rule violation (severity=error) | Exit 1, error listed |
| 3b | Run `--check validations` with only warnings | Exit 0, warnings listed |
| 4 | Run `--check validations:done-planning-checklist-no-unchecked` | Only that rule runs |
| 5 | Run `--check validations:@ticket` | Only rules for ticket entity type run |
| 6 | Run `--check all` with mixed issues | All checks run, exit 1 |
| 7 | Run `--check cardinality --check validations:rule1` | Both checks run |
| 8 | Run without --check flag | Only config validation, existing behavior |
| 9 | Run with `-o json --check all` | Valid JSON with AnalysisResult structure |
| 10 | Run with `-q --check all` | Only errors printed |

**Edge Cases:**

- No entities in project: Should pass (nothing to validate)
- No metamodel validations defined: Should pass validations check
- Invalid --check value: Clear error, exit 1
- Both config errors and check errors: Report both, exit 1
- `--check all` with no issues: Exit 0
- `--check validations:@nonexistent-type`: No rules match, exit 0 (not an error)
- Multiple `validations:` filters: Union of all matching rules

**Negative Tests:**

- `--check invalid` → Error: "unknown check type"
- `--check` without value → Error from flag parsing
- `--check validations:nonexistent-rule` → Error: "unknown validation rule"
- Project not found → Error: "project not found"

**Integration Test Approach:**

Use table-driven tests with temporary project directories:
1. Create temp dir with metamodel.yaml and test entities
2. Run validate command with various flags
3. Assert exit code and output

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing validate behavior | Low | High | Default to config-only when no --check flag |
| Performance with large graphs | Low | Medium | Reuse existing optimized workspace methods |

**Effort: M** (medium) - Straightforward extension using existing components

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] CLI help text (validate command help will update automatically via Cobra)
- [ ] User guide / reference docs - Add example for CI integration
- [ ] CLAUDE.md - No changes needed
- [ ] README.md - No changes needed

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: straightforward extension of existing patterns, no architectural decisions)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review needed)

**Design Review Findings:** N/A - Implementation reuses existing workspace methods and follows established CLI patterns.
