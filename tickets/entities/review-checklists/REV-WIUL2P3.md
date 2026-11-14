---
id: REV-WIUL2P3
type: review-checklist
title: 'Review: List-context commands receive ungated, unredacted rows on stdin'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: repo-wide runs are off for this machine; `go test -race ./internal/dataentry -run 'TestCommandExec_|TestGoldenParity'` passes, and CI on PR #2247 runs the full suite)
- [x] Lint clean (`golangci-lint run --allow-serial-runners ./internal/dataentry/...`: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: this bug adds only test comments on top of PR #2247; CI checks coverage)

## Code Review

- [x] Run `/code-review` command (rela-security-reviewer on the list-context command payload path of PR #2247; the general code review ran under TKT-F2EJQY8)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (none)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7B4FHE2 (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-2VVRP3H)

**Acceptance Status:**
- PASS: the list context reads through the same gate as `GET /api/v1/<list>` (row gate by `ReadQuery`). Evidence: `TestCommandExec_ListContextIsGatedAndRedacted` (bob sees no row); viewer golden recordings list TKT-1 to TKT-3 only.
- PASS: field redaction per row. Evidence: the hidden `status` value is absent in the unit test; the viewer golden recordings carry per-row `redacted` sets.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug; PR #2247 notes the operator-visible change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the fix ships in the existing PR #2247)
