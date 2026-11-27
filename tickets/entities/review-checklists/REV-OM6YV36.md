---
id: REV-OM6YV36
type: review-checklist
title: 'Review: Relation filter controls match on target id instead of display title'
started: "2026-11-25"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): run with CI's flags (`-timeout 20m`, no `-race`), because under `-race` `internal/dataentry` passes the default 10-minute limit; total 85.9%, all floors met

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-DQFDRXO, RR-EVMC12L, RR-L6LJZQS (design review);
RR-4CMAYI3, RR-2BKZC54, RR-2Y3O3SE, RR-KDDXSAF, RR-CRS1VRR, RR-NFJTTP1,
RR-CCTFHDW, RR-B19XQUC, RR-D2KINHI, RR-AQ9OTPX, RR-82E7C78, RR-QNLAGBX,
RR-5BP241I, RR-9I85GOF, RR-ZVOS2K9 (code review). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Two same-titled targets filter separately: PASS (`TestV1ListRelationFilter_ByID`, "same-titled target by id"; manual check with two "Backend" targets).
- Renaming a target does not change filter results: PASS (`TestV1ListRelationFilter_ByIDSurvivesRename`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-BPMRSU1

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (deferred: nothing is committed yet; the commit is made on request)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: the PR is opened after the ticket is done, on request)

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
