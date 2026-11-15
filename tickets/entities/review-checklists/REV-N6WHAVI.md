---
id: REV-N6WHAVI
type: review-checklist
title: 'Review: Lower-case sequential id_prefix can never mint a valid id'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (scoped: `go test -race` on metamodel, conflict and entitymanager; dataentry `TestValidateCreateIDOpts` and `TestGoldenParity` on default and sqlite; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint on metamodel, entitymanager and dataentry: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: run by CI; the change adds tests and no untested code)

**Comment findings.** `just comment-report` lists no finding in the touched
code. `just arch-lint` and `just plimsoll` are clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-2DDAIS2 and RR-JE23B3A (significant, addressed);
RR-3VTHTVU and RR-E0A82NU (minor, addressed); RR-0QK0BQE (minor, deferred to
TKT-MXZ6FBN); RR-860EFKX (minor, wont-fix); RR-EITMNJG (nit, wont-fix);
RR-7D2FJZN (nit, deferred). The rela-security-reviewer found nothing.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
PASS: a sequential type with `id_prefix: "nt-"` mints NT-001 and NT-002 on
fs, sqlite and postgres, and `TestCreate_MintsForNonUpperCasePrefix` pins it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; no doc states the prefix check is case-sensitive)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened by the agent workflow; CI is not awaited)
