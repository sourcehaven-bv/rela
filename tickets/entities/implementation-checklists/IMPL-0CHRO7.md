---
id: IMPL-0CHRO7
type: implementation-checklist
title: 'Implementation: Extract the seed cluster off docRuntime (36 → 33)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; the existing internal/docs suite drives create/link through Lua)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (struct embedding rejected: plimsoll v0.2.0 counts only directly-declared methods, so embedding would report 33 while leaving every seed method callable on dr — satisfying the linter without severing the reach it exists to detect)
- [x] Error handling in place (no error paths changed)

## Test Quality

- [x] ~~Fixture builders~~ (N/A: no test changes)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test changes)
- [x] ~~Only values that matter~~ (N/A: no test changes)
- [x] ~~Interpolated values from objects~~ (N/A: no test changes)
- [x] ~~Property comparisons from original object~~ (N/A: no test changes)

## Manual Verification

- [x] Feature manually tested end-to-end (`go test ./internal/...` green — the acceptance proof for a behavior-preserving refactor)
- [x] Each acceptance criterion verified (docRuntime at 33 methods, verified by count; minted id values and sequence unchanged)
- [x] ~~Edge cases manually verified~~ (N/A: the end-to-end screenshot replay needs Chrome + a built SPA and was not run; internal/docscapture passed from cache, legitimate only because ApplySeed/SeedOp are unchanged in shape and signature)

## Quality

- [x] `go build ./...` and `-tags postgres` clean
- [x] `just arch-lint` OK, `just comment-lint` clean
- [x] `just plimsoll` exit 0
- [x] `golangci-lint run internal/docs/...` — 0 issues
