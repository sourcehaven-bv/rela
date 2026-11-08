---
id: REV-6D48N5Z
type: review-checklist
title: 'Review: neoq: stop the cron goroutine on queue shutdown before tenant eviction'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: ran `go test -race ./internal/jobs/...` with and without `-tags postgres`, the only package the change touches; `go build ./...` with both tags; CI runs the full suite)
- [x] ~~Lint clean (`just lint`)~~ (N/A locally: golangci-lint on internal/jobs with `-tags postgres`, 0 issues; CI runs the full lint)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: only tests were added; CI enforces the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-GL0TQAV (significant, deferred to TKT-TD68KKT: fork
redis code rela does not use), RR-ZO6JI4E, RR-E2TAPJ6 (minor, deferred to
TKT-TD68KKT), RR-XIZ3ANP, RR-XHV6VJ5, RR-PP86MV9, RR-4LTR7AA, RR-ML17V03,
RR-DHAPO42, RR-46QOIBH (addressed), RR-4DGKVVV (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: `TestPostgresQueue_CloseReleasesGoroutines`, 20 cycles through a schema-pinned DSN, goleak clean.
2. PASS: `TestMemoryQueue_CloseReleasesGoroutines` passes on the new pin and fails on the old pin with the robfig/cron stack.
3. PASS: `TestPostgresQueue_SchemaPinnedDSN` and `TestPostgresQueue_SurvivesAnotherProcessClosing`.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore)
- [x] ~~User-facing documentation updated~~ (N/A: internal change)
- [x] ~~Docs-checklist marked as done~~ (N/A: chore)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
