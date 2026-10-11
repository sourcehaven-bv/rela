---
id: IMPL-UDH9BR
type: implementation-checklist
title: 'Implementation: Comments name capabilities, not vendor libraries or backends'
started: "2026-10-10"
completed: "2026-10-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: comments only; renamed test TestFaceSelection_FlattenedWhenUnranked covers the rename)
- [x] ~~Integration tests written~~ (N/A: no behaviour change)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: no behaviour change)
- [x] ~~Error handling in place~~ (N/A: no behaviour change)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no new tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no new tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no new tests)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no new tests)
- [x] ~~Property comparisons use original object~~ (N/A: no new tests)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: comments only)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: no behaviour change)

**Verification Evidence:**
1. Comment-stripped AST comparison against HEAD: only the 6 files with the Lowered rename differ, and only in that identifier.
2. go build with no tag, -tags postgres and -tags sqlite: pass.
3. just comment-lint: no unresolvable doc links. just lint: 0 issues.
4. grep finds no Lowered identifier for FaceSelection.
just test: all pass except TestChromeStyle_TargetsShippedClasses, which also
fails on develop locally (needs the built SPA).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: comments only)
- [x] No security issues introduced
- [x] ~~No silent failures~~ (N/A: no code paths changed)
- [x] No debug code left behind
