---
id: IMPL-T1NY8
type: implementation-checklist
title: 'Implementation: Resolve entity-ID references to titled links in Lua markdown output'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
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

- All 20 ACs covered by table-driven tests in `markdown_test.go`
(TestMdResolveRefs, TestMdResolveRefs_ListItems, TestMdResolveRefs_TableCell,
TestMdResolveRefs_EmptyMap, TestMdResolveRefs_Identity,
TestMdResolveRefs_DeepCopy, TestMdResolveRefs_TypeSetInvariant,
TestMdResolveRefs_NegativeInput, TestMdEntityRefs, TestMdEntityRefs_EmptyMeta,
TestMdEntityRefs_IntegrationWithResolveRefs, TestTitleSlug).
- Integration test `TestMdEntityRefs_IntegrationWithResolveRefs`
exercises the full `entity_refs → resolve_refs → render` flow against the
existing `mockWorkspace` harness.
- `just lint`: 0 issues.
- `just test` (race-enabled): all packages pass.
- `just coverage-check`: thresholds satisfied (73.1% total).
- `just arch-lint`: no warnings.
- AC4 (multi-backtick code spans) note: the existing parser normalizes
multi-backtick spans to single-backtick form before our walker runs, so the
rendered output uses single backticks. Content protection still holds.
Documented in the test comment.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
