---
id: REV-SM14SE
type: review-checklist
title: 'Review: fsstore accepts writes of undeclared types that vanish on reopen'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race ./internal/...`; sqlite tests on appbuild, cli and store; build and vet on the default, `postgres`, `sqlite` and `memorybackend` tags. CI runs the full suite on the PR
- [x] Lint clean (`just lint`): golangci-lint on the changed packages (0 issues), `just arch-lint`, `just plimsoll`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): storage 86.0%, fsstore 86.4%, metamodel 87.9%, appbuild 74.2%, app 57.6%, cli 61.0%, all above their floors

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
- [x] All critical review-responses addressed: the design review's critical finding (schema hot reload) added the store schema refresh
- [x] All significant review-responses addressed: all addressed in code (plural path escape, silent reload indexing, watcher gate, loader breaking existing projects, refusal wording, stray-file ownership order)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 27 review-response entities linked via
`has-review-response`: 8 from the design review, 9 from the first code review, 1
from the security review, 8 from the second code review and 1 from the final
pass. 25 are addressed. RR-CZ2DQC is wont-fix (nothing breaks today; a comment
records it) and RR-60LCF1 is wont-fix here (pre-existing; filed with the
plural-change data loss as TKT-NUMUJD).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-E7QGB8
- [x] Test evidence documented in implementation checklist: IMPL-0D0GXH

**Acceptance Status:**
- AC1 create of an undeclared type refused, nothing written: PASS
- AC2 update and conditional update refused: PASS
- AC3 family replacement keeps its existing refusal: PASS
- AC4 same through `Tx`; type change to an undeclared type writes nothing: PASS
- AC5 empty plural survives reopen: PASS
- AC6 path-like type refused: PASS
- AC7 schema reload makes a new type writable, custom directory, survives reopen: PASS
- AC8 sqlite-to-markdown export refuses undeclared types before writing: PASS
- AC9 full suite passes with test schemas from the storetest type list: PASS

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore; no user-facing docs change)
- [x] ~~User-facing documentation updated~~ (N/A: `plural:` is not described in docs/metamodel.md; the new loader errors name the type and the fix)
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
