---
id: REV-XC67Y
type: review-checklist
title: 'Review: Surface Lua errors from validation rules'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`) — validation 91.5%, validator 84.4%

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none — 0 critical findings)
- [x] All significant review-responses addressed (6 of 6 fixed in commits 4f9166a, b03c17b, 7221fa2, f0b5684, 6aeb12c)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-066E8, RR-AE4BH, RR-3U2JY, RR-MG0LG, RR-NO4VF, RR-3H1QC
(significant); RR-ZV1Z7, RR-Q7C9Y, RR-W2KL2, RR-GZEZ2, RR-TEGZP (minor);
RR-VIIZJ, RR-N3QRP, RR-JHQKY (nit) — all 14 set to `addressed`.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1-AC8 all PASS — manual verification output captured in
IMPL-YXVG1; unit tests in
`internal/validation/{lua,lua_scripterror,lua_lifecycle,lua_timeout}_test.go`
cover each AC.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing docs changes per planning checklist; "errors become visible" is a quality improvement, not an API change)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit messages explain the why, not just what (16 commits, minimal-style per project convention)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (after addressing tickets-job validation issues)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/608
