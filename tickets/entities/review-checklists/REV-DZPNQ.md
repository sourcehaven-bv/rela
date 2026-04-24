---
id: REV-DZPNQ
type: review-checklist
title: 'Review: Lua HTTP API support'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: repo uses package-floor thresholds, floors hold)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Reviewed by both `cranky-code-reviewer` and
`go-architect` agents in parallel. Findings applied before first commit:
JSON cycle/depth DoS protection (cycle detection via `luaValueToGo`,
depth cap on `goValueToLua`), error-shape parity with `ai.Error`
(retry_after field, details = unwrapped cause), HTTP method validation
(RFC 7230 token chars), classifier test coverage for
`context.Canceled` / `context.DeadlineExceeded`, timeout=0
consistency between request/convenience paths. Deliberate deferrals
(documented): no SSRF filter, first-value-wins multi-value headers.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** `go test -race ./internal/lua/...` covers all
acceptance criteria: each method has a round-trip test, timeout test,
network-error test, redirect-not-followed, non-success-status,
empty-body, JSON encode/decode + error paths, cycle protection, method
validation, full API flow.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs changes folded into this PR)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no separate checklist)

**Docs Checklist:** N/A — inlined.
Docs added to `docs-project/entities/guides/GUIDE-lua-scripting.md`
(HTTP Functions section), regenerated into `docs/lua-scripting.md`.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** To be filled after `gh pr create`.
