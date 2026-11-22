---
id: IMPL-JT2VSLV
type: implementation-checklist
title: 'Implementation: Narrow the list_entities Go path for filters that do not lower'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (fieldcond_internal_test, condtree_explained_test, LowerConditionExplained test, RowCond storetest subtest)
- [x] Integration tests written (test full flow, not just units) (hidden-field page differential on memstore, sqlite and postgres; MCP list_entities differential against a test-only Go reference)
- [x] Happy path implemented
- [x] Edge cases from planning handled (absent key, constant grants, faced types, empty-value equality, tree limits)
- [x] Error handling in place (errors surfaced, not swallowed) (refusals return a tool error naming the construct)

## Test Quality

- [x] Using fixture builders or factories for test data (newHiddenFieldFixture, newConditionFixtureOn)
- [x] No hardcoded values in assertions when object is in scope (expected sets come from the served redacted page)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built rela from the branch and drove `rela
--project=tickets mcp` over stdio:
- `entity.status == 'review' or entity.priority == 'high'` returned a page with total 154.
- `entity.status == 'ready' and entity.started < '2026-01-01'` returned an error naming `entity.started < 2026-01-01`.
- `entity.title == entity.status` returned an error naming `entity.title == entity.status`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (guard logic shared by props and the Where tree)
- [x] No security issues introduced (hidden values compared only under the visible condition; absent-key answers otherwise)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
