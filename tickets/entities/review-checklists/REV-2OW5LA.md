---
id: REV-2OW5LA
type: review-checklist
title: 'Review: Create form cannot save a pre-linked peer whose type uses a dashed id_prefix'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`; frontend vitest 3713 passed; e2e full suite passed; no Go change)
- [x] Lint clean (`just lint-frontend`, `typecheck-frontend`, `arch-lint`, `lint-md`; no Go change)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (no Go change; new frontend code has unit tests)

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

**Review Responses:** RR-ZOFENL, RR-MC0PMB (significant, addressed); RR-JKH5HB
(significant, deferred: existing backwards edges are deployment data); RR-ZVR1IY
(significant, wont-fix: the stricter permission check is correct); RR-YJR28D,
RR-BQC6Y6 (minor, addressed); RR-2LGNJV (nit, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: a section create whose peer type has a dashed id_prefix saves (e2e
create-prelink.spec.ts; unit test for `FEAT-`).
- PASS: the edge runs page --relation--> new (e2e checks the page entity's
outgoing edges; unit test checks the inverse key).
- PASS: a relation without an inverse, or an untypeable peer, links after
create with direction incoming (unit tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (run right after done, as /pr requires)

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
