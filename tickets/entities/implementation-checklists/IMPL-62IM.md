---
id: IMPL-62IM
type: implementation-checklist
title: 'Implementation: CLI validation command for CI integration'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: using workspace.NewForTest with in-memory graph is sufficient for CLI command testing)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. `rela validate` - Default config-only validation works
2. `rela validate --check cardinality` - Reports cardinality violations, exit 1
3. `rela validate --check properties` - Reports property errors, exit 1
4. `rela validate --check validations:@ticket` - Filters by entity type
5. `rela validate --check validations:rule-name` - Filters by rule name
6. `rela validate --check all -o json` - JSON output works
7. `rela validate -q --check all` - Quiet mode suppresses non-error output
8. `rela validate --check invalid` - Error for unknown check type
9. `rela validate --check validations:nonexistent` - Error for unknown rule

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
