---
id: REV-H1V7N1G
type: review-checklist
title: 'Review: Read-time today and now in every query surface'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...`; one flake in untouched `internal/cmdexec` passed on rerun)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`readtime` 100%, `filter` 90%, `predicatefns` 86%, `queryplan` 87%; CI runs `coverage-check`)

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

**Review Responses:** design review RR-5X1BFBE, RR-7LRJ1TO, RR-OCS1CUV,
RR-60X9AJ3, RR-GER2W7B, RR-VSG6VJP; code review RR-X9JS85M, RR-OMB2S0J,
RR-LSN4FVG, RR-2FW567B, RR-F0OWC8U, RR-05D091O, RR-VR56SES, RR-9ZW8BN1
(addressed), RR-PAH4JDD, RR-7M4F7J2, RR-8YRYPY3, RR-ICNI5PW, RR-ZXQ53GR (minor,
deferred). The two critical design findings are deferred to TKT-4Z2BIRG: this
ticket does not lower date comparisons, so the risk they describe does not exist
yet.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- `$today` in a dashboard `query:` and in search: PASS (`TestV1Search_DateVariables`)
- One read time per request at the edge: PASS (`TestAttachReadTime`, `TestBinders_ReadTime`, `TestTaskContext_FreshReadTime`)
- date vs today, datetime vs now, date vs datetime: PASS (`TestReadTimeConditions`)
- Same rows on fs, sqlite and postgres: PASS (`TestListCondition_DateRows`)
- Offsets `$today±Nd/w`: PASS (`TestResolveDateVariable`, `TestFromFilterDateVariables`)
- Docs examples corrected: PASS (guides updated, `just docs` regenerated)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-DD8HLA5

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run the pr command~~ (N/A: the pull request is opened after `done`, per the task brief)

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
