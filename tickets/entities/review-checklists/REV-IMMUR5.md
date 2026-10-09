---
id: REV-IMMUR5
type: review-checklist
title: 'Review: gate MCP counts through the read ACL'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./internal/...`, plus `-tags sqlite` for visibility and mcp
- [x] Lint clean (`just lint`) — golangci-lint, arch-lint, plimsoll
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: every new branch has a test; CI runs the coverage gate)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review responses:** RR-8YAE1J (deferred to TKT-OJ1UYJ), RR-WOSDYI, RR-CRZZP4,
RR-D16E0Q, RR-FBU79L, RR-GMDGL2, RR-SA13CX, RR-HS49Z2 (deferred to TKT-OJ1UYJ),
RR-TZFZY9.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
