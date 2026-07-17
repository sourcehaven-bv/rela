---
id: REV-FGMBJ
type: review-checklist
title: 'Review: Extract attachment route cluster into attachmentHandler'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./internal/dataentry/...`)
- [x] Lint clean (`golangci-lint run internal/dataentry/... internal/cli/...`) — 0 issues
- [x] `gofmt` clean
- [x] `just plimsoll` passes — `App` directive ratcheted 143 → 131; cli pin corrected 29 → 30 (develop was red via #1142)
- [x] `just arch-lint` passes
- [x] Builds across default / `postgres` / `memorybackend` tags

## Manual Review

- [x] Self-reviewed the diff for unrelated changes
- [x] Swappable collaborators (acl/audit/fields/runner) are closures — attachment ACL tests reassign `app.acl` and `app.attachmentRunner` post-construction and still take effect
- [x] `writeMu` shared by pointer — uploads/deletes serialize with all other mutation handlers (race detector guards)
- [x] Uniform-404 invariant preserved — `gateRead` delegates to App's shared `gateReadOrNotFound` (RR-NGMI)
- [x] Capture-once improved: PUT serializes with its own snapshot's `s.Meta`; property checks take the snapshot explicitly
- [x] ~~`/code-review` command~~ (N/A: mechanical method-move refactor, no behavior change; the attachment test suite — incl. ACL, scan-reject, size-cap cases — passes unchanged)
- [x] ~~Critical/significant review-responses addressed~~ (N/A: no review run)

**Review Responses:** N/A — mechanical extraction, behavior-preserving.
