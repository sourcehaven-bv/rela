---
id: IMPL-N701G
type: implementation-checklist
title: 'Implementation: Enable additional golangci-lint v2 linters for high-signal checks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: tooling-config PR, no new production logic)
- [x] ~~Integration tests written~~ (N/A: `just ci` is the integration test)
- [x] Happy path implemented (9 linters enabled; --fix + hand-fixes land)
- [x] Edge cases from planning handled (`contextcheck` deferred with note; testutil/test-file scoped exclusions added)
- [x] Error handling in place

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (`just ci` exits 0)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- `golangci-lint run ./...` → `0 issues.`
- `just test` → all packages pass
- `just ci` → exits 0 (lint + test + coverage-check + build + docs-check)
- Full --fix pass moved 317 findings → 34; remaining 34 addressed via code fixes (intrange, whitespace, two stderrors.New reverts) or scoped nolint/exclusion (2 containedctx production structs with real-lifetime ctx, forcetypeassert in test files, usetesting in testutil).

## Quality

- [x] Code follows project patterns
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind
