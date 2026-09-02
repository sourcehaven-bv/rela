---
id: IMPL-CLLMB1
type: implementation-checklist
title: 'Implementation: mcp.Server round 2: lua/schema/resources/prompts handlers (38 → ~25)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; dispatch/golden/capabilities-posture suites pin the surfaces)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (toolGetSchema/toolGetMetamodel aliasing onto the same handler preserved on the new receiver)
- [x] Error handling in place

## Test Quality

- [x] ~~Fixture builders~~ (N/A: test changes are mechanical re-points)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only values that matter~~ (N/A)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons from original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (full `go test ./...` green)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence (PR #1468, branch tkt-mgne5l-mcp-round2):**
- Server 38 → **25**; directive ratcheted to match.
- `luaHandler` isolates the Lua deps (WriteDeps/cache/projectRoot) that no
other handler uses, so no other handler can now reach them;
`schemaResourceHandler` merges schema tools + resources (identical store+meta
deps); `promptHandler` takes store+meta+tracer+typeResolver.
- Coordinator fixes after the implementing agent was interrupted: the
shared derivation returned 6 positional values (tripping gocritic's
tooManyResultsChecker); refactored to a named `handlerSet` struct **embedded**
on Server, so every `s.trace.…` reference still resolves via field promotion and
NewServer/test-helper wiring still cannot drift. Also fixed a doclink in the new
godoc (Go cannot link the unexported `Deps.handlers`, so the brackets were
dropped).
- Gates: build, full `go test ./...`, `-race ./internal/mcp/`, plimsoll,
arch-lint, comment-lint, golangci-lint (0 issues) — all green.

## Quality

- [x] Code follows project patterns (mirrors the TKT-YUETL7 handler shape on the base branch)
- [x] Checked for DRY opportunities (one shared `Deps.handlers` derivation for NewServer + test helpers, so wiring can't drift)
- [x] No security issues introduced (principalMiddleware untouched; no principal threaded into any handler struct — identity still read from ctx)
- [x] No silent failures
- [x] No debug code left behind
