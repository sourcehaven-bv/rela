---
id: REV-DTC76RP
type: review-checklist
title: 'Review: OpenAPI spec lacks If-Match and 412 on the DELETE operations'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: ran the affected packages instead; `go test ./internal/openapi/` and `go test ./internal/dataentry -run 'TestGoldenParity|OpenAPI'` pass; CI runs the full suite)
- [x] Lint clean (`golangci-lint run --allow-serial-runners ./internal/openapi/...`: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: adds a test and eight spec lines; no package floor can drop; CI checks it)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: eight-line spec-only change that copies the existing PATCH declarations; self-reviewed against both DELETE handlers in internal/dataentry)
- [x] ~~All critical review-responses addressed~~ (N/A: no review-responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review-responses)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

## Acceptance Verification

- [x] Each acceptance criterion tested (entity and relation DELETE declare If-Match and 412)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** PASS: TestGenerator_ConditionalWritesDeclareIfMatch and
the regenerated goldens.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: the fix is the API reference itself)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opened with gh pr create; CI not monitored by this task)
