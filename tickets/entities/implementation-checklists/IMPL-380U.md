---
id: IMPL-380U
type: implementation-checklist
title: 'Implementation: Add Markdown AST API to Lua scripting'
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

Test scenarios verified via unit tests in `internal/lua/markdown_test.go`:
- AC1-3: parse, render, round-trip stability
- AC4-5: shift_headers, set_min_header_level with clamping
- AC6-7: headers extraction with level filtering
- AC8: extract_section with nested content
- AC9: first_paragraph extraction
- AC10: concat multiple ASTs
- AC11-15: All node constructors (heading, paragraph, code_block, thematic_break, blockquote, list)

Edge cases tested:
- Empty content, no headers, headers at level 1/6 with clamping
- Code block containing "# not a header" preserved
- Unicode content (Japanese, emojis)
- Error cases: nil/invalid arguments

Integration test: `TestMdIntegrationWithEntity` verifies entity content parsing

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
