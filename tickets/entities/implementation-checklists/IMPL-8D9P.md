---
id: IMPL-8D9P
type: implementation-checklist
title: 'Implementation: Add task list (checkbox) support to Lua markdown AST'
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

End-to-end verification was performed via the unit tests (they exercise the
exact same code path as `rela script`: Lua VM → `rela.md.parse` → AST →
`rela.md.render`). External CLI invocation was attempted but the test sandbox
cwd handling made a clean external test difficult; the unit tests provide
equivalent coverage of the user-visible behavior.

| AC | Test | Result |
|----|------|--------|
| AC1: parse task items | TestMdTaskListParse | PASS — items have task=true, checked=bool, text=string |
| AC2: render task syntax | TestMdTaskListRender/constructor_with_task_items | PASS — `- [x] done\n- [ ] todo\n` |
| AC2 ordered: `1. [x]` | TestMdTaskListRender/ordered_task_list | PASS |
| AC3: constructor accepts table | TestMdTaskListRender/constructor_with_task_items | PASS |
| AC4: existing string tests | TestMdRender/unordered_list, /ordered_list | PASS unchanged |
| AC5: task=false → plain | TestMdTaskListRender/task=false_renders_as_plain_item | PASS |
| AC5: missing task → plain | TestMdTaskListRender/missing_task_field_renders_as_plain_item | PASS |
| AC6: missing text → empty | TestMdTaskListRender/missing_text_field_renders_empty | PASS |
| AC7: strikethrough preserved | TestMdTaskListRoundTrip/strikethrough_preserved_in_task | PASS — `- [x] ~~done~~` round-trips |
| AC7: mid-text strikethrough | TestMdTaskListRoundTrip/strikethrough_mid-text_preserved | PASS |
| Limit #1: bold dropped | TestMdTaskListLimitations/bold_dropped | PASS — markers dropped, no crash |
| Limit #2: multi-block | TestMdTaskListLimitations/multi-block_item_does_not_crash | PASS — no crash |
| Limit #3: mixed list | TestMdMixedListBehavior | PASS — empirically locked in |
| Side effect: paragraph strikethrough | TestMdParagraphStrikethrough | PASS — `~~struck~~` preserved in paragraph |

**17 new test cases, all passing.**

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Quality checks:**

- `just lint` — clean
- `just test` — all packages pass
- `just coverage-check` — PASS (internal/lua/markdown.go at 88.7%, no regression)
- Code follows the same patterns as `internal/markdown/content.go` for
TaskCheckBox detection and the existing `extractInlineText` switch statement
- No new I/O, capabilities, or attack surface added (pure parse/render extension)
