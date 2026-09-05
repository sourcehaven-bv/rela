---
id: IMPL-0WWHV6
type: implementation-checklist
title: 'Implementation: Runtime under the load line: extract elevation/output/schema-sort clusters (45 → ~37)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; existing bypass_acl/elevated/output/schema suites drive every moved binding through Lua)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (elevation semantics moved verbatim; `registerBindings` still builds an elevationBindings only when an elevated handle was wired, preserving the structural-absence guarantee — an unwired capability is ABSENT, not present-and-erroring)
- [x] Error handling in place

## Test Quality

- [x] ~~Fixture builders~~ (N/A: no test changes)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only values that matter~~ (N/A)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons from original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (full `go test ./...` green)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence (PR #1467, branch tkt-yvreqn-lua-under-line):**
- Runtime 45 → **37 — under the 40-method load line**, the arc goal for this
type. Directive pinned at 37 (kept, not deleted, so the gain can't erode).
- New files: elevation.go (`elevationBindings`), output.go
(`outputBindings`), schemasort.go (`schemaBindings` + `luaSortEntities` as a
free function since it touches no runtime state).
- Gates re-run and verified by the coordinator after the implementing agent
was interrupted mid-cycle: build, full `go test ./...`, `-race ./internal/lua/`,
plimsoll, arch-lint, comment-lint, golangci-lint (0 issues) — all green.

## Quality

- [x] Code follows project patterns (urlHelpers/mdHelpers precedent; narrow consumer-side deps per binding type)
- [x] Checked for DRY opportunities
- [x] No security issues introduced (elevation is receiver plumbing only: same recorder calls, same object-capability scoping, same withheld-capability semantics)
- [x] No silent failures
- [x] No debug code left behind
