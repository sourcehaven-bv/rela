---
id: IMPL-FYOH5S
type: implementation-checklist
title: 'Implementation: Extract markdown AST helpers off lua.Runtime (plimsoll ratchet 105 → ~60)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; existing Lua-level suite covers every moved binding)
- [x] ~~Integration tests written~~ (N/A: same — markdown_test.go drives all bindings through real Lua scripts)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil-meta / nil-reader raises preserved verbatim; ctx stays a closure so timeout propagation is unchanged; m.ls/c.ls == Runtime.L by construction in registerMarkdownModule)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no test changes)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test changes)
- [x] ~~Only specifying values that matter~~ (N/A: no test changes)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no test changes)
- [x] ~~Property comparisons use original object~~ (N/A: no test changes)

## Manual Verification

- [x] Feature manually tested end-to-end (full `go test ./...` green — the acceptance proof for a behavior-preserving refactor)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `just plimsoll` passes with Runtime ratcheted 105 → 60. Bonus: the linter
rejected the first cut (mdHelpers at 42 > 40 for a new type), which forced the
mdASTConverter split — mdHelpers 26 / mdASTConverter 16 / mdEntityRefs 3, all
under the line with no new directives.
- `go test ./...` fully green; `go test -race ./internal/lua/` green.
- `just arch-lint`, `just comment-lint`, `just lint` (golangci-lint): clean.

## Quality

- [x] Code follows project patterns (urlHelpers precedent; consumer-side narrow deps for mdEntityRefs)
- [x] Checked for DRY opportunities (pure moves; no new duplication introduced)
- [x] No security issues introduced (entity_refs still reads through the ACL-gated VisibleReader with callerCtx; RR-ZA452J comment moved with the code)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
