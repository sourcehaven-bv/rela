---
id: REV-DI8V8
type: review-checklist
title: 'Review: Extract sync route cluster into syncHandler'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./internal/dataentry/...`)
- [x] Lint clean (`golangci-lint run internal/dataentry/...`) — 0 issues
- [x] `gofmt` clean
- [x] `just plimsoll` passes — `App` directive ratcheted 170 → 154
- [x] `just arch-lint` passes
- [x] Builds + vet across default / `postgres` / `memorybackend` tags

## Manual Review

- [x] Self-reviewed the diff for unrelated changes
- [x] Consumer-side interfaces (`syncStore`, `syncDeleter`) declared at the call site per CLAUDE.md
- [x] `writeMu` shared by pointer — sync writes still serialize with other mutation handlers (race detector guards)
- [x] Optional capabilities resolved once in `newSyncHandler`; nil on fs/memory builds (endpoints degrade to 501)
- [x] ~~`/code-review` command~~ (N/A: mechanical method-move refactor, no behavior change; verified by the unchanged, still-passing sync test suite)
- [x] ~~Critical/significant review-responses addressed~~ (N/A: no review run)

**Review Responses:** N/A — mechanical extraction, behavior-preserving.
