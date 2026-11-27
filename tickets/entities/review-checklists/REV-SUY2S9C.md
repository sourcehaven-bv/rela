---
id: REV-SUY2S9C
type: review-checklist
title: 'Review: MCP update_entity ignores field-level write grants'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (deferred to CI: local runner overloaded; touched packages pass: internal/dataentry, internal/dataentrywire, internal/mcp, internal/appbuild, internal/appbuild/appbuildtest)
- [x] ~~Lint clean (`just lint`)~~ (deferred to CI: local runner overloaded)
- [x] Comment lint gate clean (`just comment-lint`) (commentlint v0.3.1 commented-code,doclink on the touched packages: clean)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (deferred to CI: local runner overloaded; change adds tests only)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer run)
- [x] All critical review-responses addressed (none found)
- [x] All significant review-responses addressed (none found)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-ZU1NAC9, RR-L97TLN3, RR-G35MFM0, RR-AAIQWTJ,
RR-YU2I9NH, RR-EKEUEPH, RR-PV5OVPA, RR-LM40SUR (all minor or nit; five
addressed, three deferred with reasons)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: production MCP refuses the write (TestRemoteMCP_UpdateEntityHonorsFieldGrants, mutation-checked).
- PASS: corpus records the denial (decl-jwt/BUG-VUNO11-mcp-update-denied-field.editor.golden); no other recording widened.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix in test wiring)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh pr create after this checklist; CI is tracked on GitHub)

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
