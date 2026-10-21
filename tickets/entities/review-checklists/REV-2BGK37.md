---
id: REV-2BGK37
type: review-checklist
title: 'Review: fsstore temp-file cleanup can delete another process''s in-flight write'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on storage, fsstore, analysis, fsimport, cli, project and app; `go test ./internal/...`; build and vet on the default, `postgres`, `sqlite` and `memorybackend` tags. CI runs the full suite on the PR
- [x] Lint clean (`just lint`): golangci-lint on the changed packages (0 issues), `just arch-lint`, `just plimsoll`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): storage 86.4%, fsstore 85.4%, analysis 90.5%, fsimport 74.6%, all above their floors

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer ran on the diff
- [x] All critical review-responses addressed: none raised
- [x] All significant review-responses addressed: the two significant design-review findings changed the plan before implementation; the code reviews raised none
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 12 review-response entities linked via
`has-review-response`: 6 from the design review, 5 from the code review and 1
from the security review. All 12 are addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-K8317C
- [x] Test evidence documented in implementation checklist: IMPL-I103B2

**Acceptance Status:**
- AC1 concurrent writes stay whole and succeed: PASS (fails on develop)
- AC2 mode is the requested mode with the umask: PASS
- AC3 sweep keeps fresh and removes stale temp files, real files kept: PASS
- AC4 failed rename leaves no temp file: PASS
- AC5 scan and watcher ignore temp names: PASS
- AC6 `rela gc --temp-files` reports only stale leftovers: PASS
- AC7 fsimport skips temp files under `.rela/`: PASS
- AC8 concurrent attachment uploads of the same name: PASS (fails on develop)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore; no user-facing docs change)
- [x] ~~User-facing documentation updated~~ (N/A: only the `rela gc --temp-files` help text changed, which is in the diff)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist for a chore)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed (none added in the diff)
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (opened after the ticket reaches `done`; see TKT-UFV01M)

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
