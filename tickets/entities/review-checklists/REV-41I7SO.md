---
id: REV-41I7SO
type: review-checklist
title: 'Review: Entity-anchored documents skip the face gate: a face-restricted reader can render a hidden face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): verified on the shared branch with BUG-PLZDPR
- [x] Lint clean (`just lint`): 0 issues
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): PASS

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): run for BUG-PLZDPR on the same branch; the security reviewer raised this gap
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-TZ4U5F)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-TZ4U5F (addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-OSMCIH)

**Acceptance Status:**

PASS: a face the caller may not read is the uniform 404 on both document routes,
and the renderer does not run (`TestFaceGrant_AnchoredDocumentIsFaceGated`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: behavior now matches the documented face gate)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: the commit is made with /pr, after done)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: `/pr` runs after done; see TKT-UFV01M)

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
