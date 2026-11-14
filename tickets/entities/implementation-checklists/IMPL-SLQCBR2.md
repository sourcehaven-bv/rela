---
id: IMPL-SLQCBR2
type: implementation-checklist
title: 'Implementation: Indented --- line closes frontmatter early and truncates properties'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestSplit: indented delimiter is content; closing delimiter tolerates trailing whitespace)
- [x] Integration tests written (test full flow, not just units) (fuzz seed 03748c2413d59a63 runs the fsstore create/get round trip)
- [x] Happy path implemented
- [x] Edge cases from planning handled (trailing whitespace and CRLF on the closing fence still accepted)
- [x] ~~Error handling in place~~ (N/A: pure string split with no error path)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: table of literal file contents is the subject under test)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object~~ (N/A: fuzz target already compares against the written entity; unit test compares split strings)

## Manual Verification

- [x] Feature manually tested end-to-end (FormatDocument then ParseDocument of a "---\n" key and of "a\n---\nb" values: truncated before the fix)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: bug fix; the criterion is the fuzz round trip, covered by the seed)
- [x] Edge cases manually verified (45s fuzz of FuzzPropertyValuesTypeZoo and 20s of FuzzSplit, no failures)

**Verification Evidence:** go test for frontmatter, markdown, store/...,
analysis, conflict, fsimport, templating: all pass. golangci-lint on
internal/frontmatter: 0 issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (single helper isClosingDelimiter; no repetition)
- [x] No security issues introduced
- [x] ~~No silent failures~~ (N/A: no error path; the fix removes a silent truncation)
- [x] No debug code left behind (temporary probe test deleted)
