---
id: REV-4VGWWP
type: review-checklist
title: 'Review: pgstore change-feed listener fails when RELA_DATABASE_URL sets pool_max_conns'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just ci`; postgres-tagged pgstore suite with `-race`; docscapture with a pool-tuned RELA_DATABASE_URL)
- [x] Lint clean (`just lint`, `just arch-lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check` within `just ci`)

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

- [x] Run `/code-review` command (cranky-code-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-Z24MW8 is an older defect outside this diff, deferred to BUG-BR9CXF)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-Z24MW8 (deferred), RR-6NGPLV, RR-G1OQCD, RR-0DZGLW,
RR-P1T2P0, RR-PUYN03

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-QK6RNK)

**Acceptance Status:**

- PASS: the listener connects with a DSN carrying `pool_*` keys
(`TestCrossProcessPropagation_PoolTunedDSN`; failed before the fix).
- PASS: `listenerConnConfig` strips pool keys and keeps server parameters
(`TestListenerConnConfig_StripsPoolKeys`, no database).
- PASS: `rela-server-postgres` with `pool_max_conns=2` starts without the
"change feed unavailable" warning; the old build logs it.
- PASS: the `docscapture` admin connection accepts a pool-tuned DSN.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix, no user-facing change)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A here: the commit follows this checklist)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: /pr runs after the bug is done; PR and CI status are recorded on GitHub per TKT-UFV01M)

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
