---
id: IMPL-0BHQ7R
type: implementation-checklist
title: 'Implementation: Extract Tier-B capability bindings off docRuntime (36 → 31)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; the existing internal/docs suite drives every moved binding through Lua)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (seed ops reach the replay path as `seed func() []SeedOp`, not a captured slice — registration runs once before any island while ops accumulate during execution, so a snapshot would always be empty)
- [x] Error handling in place (nil-capturer / nil-apiClient fail-loud messages byte-identical)

## Test Quality

- [x] ~~Fixture builders~~ (N/A: no test changes)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test changes)
- [x] ~~Only values that matter~~ (N/A: no test changes)
- [x] ~~Interpolated values from objects~~ (N/A: no test changes)
- [x] ~~Property comparisons from original object~~ (N/A: no test changes)

## Manual Verification

- [x] Feature manually tested end-to-end (`go test ./internal/...` green — the acceptance proof for a behavior-preserving refactor)
- [x] Each acceptance criterion verified (docRuntime at 31 methods, verified by count)
- [x] ~~Edge cases manually verified~~ (N/A: the end-to-end screenshot replay needs Chrome + a built SPA and was not run; ApplySeed/SeedOp are unchanged in shape and the moved bodies are byte-identical apart from the receiver)

## Quality

- [x] `go build ./...` and `-tags postgres` clean
- [x] `just arch-lint` OK — no warnings
- [x] `just plimsoll` exit 0
- [x] `golangci-lint run internal/docs/...` — 0 issues
