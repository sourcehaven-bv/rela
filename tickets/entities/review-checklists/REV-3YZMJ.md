---
id: REV-3YZMJ
type: review-checklist
title: 'Review: Extract command route cluster into commandHandler'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./internal/dataentry/...`)
- [x] Lint clean (`golangci-lint run internal/dataentry/...`) — 0 issues
- [x] `gofmt` clean
- [x] `just plimsoll` passes — `App` directive ratcheted 154 → 143
- [x] `just arch-lint` passes
- [x] Builds + vet across default / `postgres` / `memorybackend` tags

## Manual Review

- [x] Self-reviewed the diff for unrelated changes
- [x] Collaborators are narrow closures over App (consumer-side), mirroring `affordanceService`
- [x] Routing behavior unchanged — same endpoints mounted; `handleOpenURL` stays a tested-but-unmounted method as before
- [x] `resolveCommands` moved with the cluster; `handleV1Commands` delegates via `a.commands.resolveCommands`
- [x] Handler owns no mutable state — in-flight exec registry stays a package-level `sync.Map`
- [x] ~~`/code-review` command~~ (N/A: mechanical method-move refactor, no behavior change; verified by the unchanged, still-passing command test suite)
- [x] ~~Critical/significant review-responses addressed~~ (N/A: no review run)

**Review Responses:** N/A — mechanical extraction, behavior-preserving.
