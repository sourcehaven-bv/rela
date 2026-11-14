---
id: REV-TO9RFWE
type: review-checklist
title: 'Review: Drop stale realtimelimits allowance for channels in arch-lint'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: lint config only; ran `go build ./...` and `go test` on channels and realtime, both pass; full suite runs in CI)
- [x] ~~Lint clean (`just lint`)~~ (N/A: no Go code changed; `golangci-lint` on internal/channels and `just arch-lint` clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: two-line deletion in lint config, self-reviewed)
- [x] ~~All critical review-responses addressed~~ (N/A: no review-responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review-responses)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: the entry is gone and `just arch-lint` reports no warnings.
2. PASS: the `channels.MaxBatch` godoc and the config now agree.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: chore)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

**Docs Checklist:** N/A

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: minimal commit message by convention; the ticket and the PR link to #2235)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
