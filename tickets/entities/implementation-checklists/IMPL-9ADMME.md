---
id: IMPL-9ADMME
type: implementation-checklist
title: 'Implementation: Extract HTTP/cache/AI binding clusters off lua.Runtime (ratchet 60 → ~45)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; http_test.go/cache/ai suites drive every moved binding through Lua)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (scriptPath held as `func() string` closure — godoc explains SetScriptPath/RunFile mutate after registration, a captured string would freeze the cache namespace at "")
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Fixture builders~~ (N/A: no test changes)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test changes)
- [x] ~~Only values that matter~~ (N/A: no test changes)
- [x] ~~Interpolated values from objects~~ (N/A: no test changes)
- [x] ~~Property comparisons from original object~~ (N/A: no test changes)

## Manual Verification

- [x] Feature manually tested end-to-end (full `go test ./...` green — the acceptance proof for a behavior-preserving refactor)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence (from the implementing agent, PR #1462, commit
9084dfbc):**
- Runtime 60 → 45 verified by grep; directive ratcheted with history line;
TODO count updated.
- Local gates all green: build, ./internal/lua tests, full suite, `-race` on
internal/lua, plimsoll, arch-lint, comment-lint, golangci-lint.
- CI green on all jobs except Rela Tickets (expected — resolved when this
ticket's files land on the branch).
- Deviation from plan (improvement): `httpBindings` is an empty stateless
struct and `aiBindings`/`cacheBindings` hold no Lua-state field — every binding
receives its `*lua.LState` as an argument, and an unread field would trip
`unused`. Full urlHelpers parity, strictly better than the planned state-holding
shape.

## Quality

- [x] Code follows project patterns (urlHelpers/mdHelpers precedent; cacheStore consumer-side interface reused)
- [x] Checked for DRY opportunities (httpContext/chatContext narrowed to *lua.LState — removes the Runtime coupling instead of duplicating it)
- [x] No security issues introduced (caps.HTTP and caps.AI gates unmoved in registerBindings)
- [x] No silent failures
- [x] No debug code left behind
