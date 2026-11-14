---
id: IMPL-YL8P65Z
type: implementation-checklist
title: 'Implementation: Drop stale realtimelimits allowance for channels in arch-lint'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no Go code changed; arch-lint is the check)
- [x] ~~Integration tests written~~ (N/A: lint config only)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: none identified)
- [x] ~~Error handling in place~~ (N/A: no code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests added)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: none identified)

**Verification Evidence:** `just arch-lint`: OK, no warnings. `go build ./...`
passes. `go test ./internal/channels/ ./internal/realtime/` passes.
`golangci-lint run ./internal/channels/...`: 0 issues. `just comment-lint`:
clean. The godoc on `channels.MaxBatch` ("which channels may not import") now
matches the config.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: deletion only)
- [x] No security issues introduced
- [x] ~~No silent failures~~ (N/A: no code)
- [x] No debug code left behind
