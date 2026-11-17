---
id: REV-UE92L4I
type: review-checklist
title: 'Review: Mac conventions in rela-desktop: reopen after closing the last window, Find, Print, Show in Finder, Help'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test -race ./cmd/rela-desktop and vitest on the changed files locally; the full suite runs in CI on the PR)
- [x] Lint clean (`just lint`) (golangci-lint on cmd/rela-desktop: 0 issues; prettier clean on the changed SPA files)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (new logic has unit tests; Wails-only paths carry coverage-ignore with a reason)

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

**Review Responses:** RR-IYILI1Q, RR-O2ZWDLU (significant, addressed);
RR-L1CE6SS, RR-XZC80MB, RR-2H7BPYH, RR-B6JXL9Q, RR-F1NDL57, RR-PR1NTC3,
RR-S1ZEDR1 (minor, addressed); RR-POY1OAE (minor, deferred); RR-7P17AM4,
RR-MIEZCV4 (nit, addressed); RR-5L40604 (nit, wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all PASS, by hand in a production build (evidence in
IMPL-UHE9AL7): reopen after closing the last window (Dock, Open Recent, Finder),
Find, Print, Show in Finder (disabled during Clone from Git), Rela Help, and the
saved window frame on Close Window and on Quit.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-AGXMLNI

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR 2281 already exists; CI runs on the push)

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
