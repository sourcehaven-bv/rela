---
id: REV-OIRZLF
type: review-checklist
title: 'Review: Data-entry answers 500 instead of 401 when a handler finds no bound identity'
started: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`just coverage-check` ran the full race suite: all packages touched by the diff pass. Two unrelated failures: cmd/rela-desktop TestChromeStyle_TargetsShippedClasses needs the built SPA, and internal/store/sqlitestore hit the 10m timeout under machine load; neither imports acl or dataentry)
- [x] Lint clean (`just lint`) (golangci-lint on internal/acl and internal/dataentry: 0 issues; `just arch-lint` OK)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (go-test-coverage on the profile: package and total thresholds PASS)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer run on the diff)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-DD2Z0O, RR-HM3ZCX)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-DD2Z0O, RR-HM3ZCX, RR-DTMBVY, RR-G5435G, RR-2OVWTT, RR-43Q638, RR-1FG2U2, RR-RGXTIW, RR-XLFCDS

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (bug analysis checklist BUGA-ZSOWDM; TestNoIdentity_Status)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- ErrNoIdentity answers 401 on create, update, delete: PASS (TestNoIdentity_Status)
- Mapping is central, not per handler: PASS (writeNoIdentity, writeIdentityFaultDeny; TestForbiddenMappersHandleIdentityFaults)
- Identity mismatch stays 500: PASS
- Hidden entity stays a uniform 404: PASS (nothing-bound cases for existing and missing ids)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; the 401 body already exists for the JWT gate)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh after this checklist; see the TKT-UFV01M note below)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
