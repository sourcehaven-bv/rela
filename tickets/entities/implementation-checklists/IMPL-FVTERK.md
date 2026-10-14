---
id: IMPL-FVTERK
type: implementation-checklist
title: 'Implementation: Escape feed text in entity bodies'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestEscapeMarkdown, TestRun_TextIsEscapedOnlyInBodies, link and Kind cases in source_test.go)
- [x] Integration tests written (test full flow, not just units) (TestSubscription_EndToEnd asserts the stored body escapes a title with image markup and the property keeps it)
- [x] Happy path implemented
- [x] Edge cases from planning handled (existing backslash, entity references, newlines flattened first, NUL, markdown in append content)
- [x] Error handling in place (errors surfaced, not swallowed) (no new error paths)

## Test Quality

- [x] Using fixture builders or factories for test data (mapSource prefixes select the kind)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** The appbuild e2e test runs the real scheduler, queue
and write path against an httptest feed whose item title is `Second
![](https://t.example/p.gif)`. The stored body holds `Second
\!\[\](https://t.example/p.gif)` and the title property is unchanged. The
dataentry webhook suite passes unchanged (values stay raw).

## Quality

- [x] Code follows project patterns (check similar code) (single-pass strings.Replacer like transform.EscapeInline)
- [x] Checked for DRY opportunities: one escaper in ingest; refs and blockRefs merged into one kind table
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
