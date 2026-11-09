---
id: REV-Q349FW1
type: review-checklist
title: 'Review: Flows M1d: run flows end to end'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on every changed package (fs), plus the flow, flowrun, fsimport and cli tests under the `sqlite` and `postgres` tags; the full suite runs in CI
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, 0 issues; `just arch-lint` clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed (RR-FH3ZC9E is wont-fix with a reason)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-1LNUTMF, RR-5VHCYVA (critical, addressed); RR-OHQQ0HZ,
RR-BADQJ16, RR-3NYOU8A, RR-FAJ21U5, RR-RNIBW94 (significant, addressed);
RR-FH3ZC9E (significant, wont-fix); RR-0MZ9CG3, RR-RDRLZX9, RR-S0AWFF1 (minor,
addressed); RR-GCDTUIW, RR-3TAZGJL (minor, deferred to TKT-FWQZGAM); RR-RQQ4GE0,
RR-QYWJYAT (nit, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- The sample flow runs end to end on file, pg and sqlite: PASS. `TestFlowControl_GatesStartCancelAndRuns` and the flowruntime tests start, list and cancel the sample flow on all three backends; manual runs on fs and on sqlite after `rela db import-fs`.
- Carried-over findings: each is addressed or deferred with a reason on the ticket (Decisions section).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ADEQTW4

## Final Checks

- [x] Commit message explains the why, not just what: minimal by the user's instruction; the ticket id carries the why
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: the PR is opened after the ticket is done
