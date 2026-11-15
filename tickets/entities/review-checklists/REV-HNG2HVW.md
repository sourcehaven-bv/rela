---
id: REV-HNG2HVW
type: review-checklist
title: 'Review: Run the full race suites after merge, not in the merge queue'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: no Go code changed; CI on #2348 runs the suites)
- [x] ~~Lint clean (`just lint`)~~ (N/A: no Go code changed; actionlint, shellcheck and markdownlint clean)
- [x] ~~Comment lint gate clean~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained~~ (N/A: no Go code changed)

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

**Review Responses:** RR-NEDPI23 (significant, addressed), RR-WXKVSHW,
RR-93R5TOB, RR-MWNK2Q0 (minor, addressed), RR-NDM9HAY (nit, addressed),
RR-8NCGO3B, RR-NFHXRQ1 (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: the race gate script ran green locally; the PR run on #2348 exercises the PR path.
2. Checked by reading the workflow: `GOFLAGS=-race` on push. The first push run after merge confirms it.
3. PASS by inspection: the alert job reuses the security.yml dedup pattern.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created~~ (N/A: chore, not an enhancement)
- [x] ~~User-facing documentation updated~~ (N/A: internal CI change; CLAUDE.md updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
