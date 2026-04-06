---
id: IMPL-9VAI
type: implementation-checklist
title: 'Implementation: Add GFM table parsing and serialization to Lua markdown AST'
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
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: test data is inline markdown, hardcoded expected values are appropriate for parse tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. **Structured table nodes**: TestMdTableParse/simple_table verifies type, header, rows, alignments — PASS
2. **Render back to markdown**: TestMdTableRender verifies GFM output with pipes and separators — PASS
3. **Round-trip stability**: TestMdTableRoundTrip parse→render→parse→render produces identical output — PASS
4. **Existing functions work**: TestMdTable (existing test for rela.md.table()) still passes, TestMdEntityTable_* all pass — PASS
5. **Mixed content**: TestMdTableParse/mixed_content_with_table verifies heading+table+paragraph — PASS
6. **Alignment preservation**: TestMdTableParse/alignment_markers verifies left/center/right — PASS
7. **Header only table**: TestMdTableParse/header_only_table — PASS
8. **Inline formatting**: TestMdTableParse/inline_formatting_in_cells extracts plain text — PASS
9. **Missing header graceful**: TestMdTableRender/render_missing_header_gracefully produces empty output — PASS

`just lint` — clean `just test` — all pass `just coverage-check` — PASS

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
