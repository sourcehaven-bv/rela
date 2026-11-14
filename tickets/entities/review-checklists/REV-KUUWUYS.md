---
id: REV-KUUWUYS
type: review-checklist
title: 'Review: Declarative mapped import: rela import --mapping (JSON/XML/YAML/CSV, diff, Lua hook)'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (go test on mapimport, cli, projectsetup and importer; full suite runs in CI)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages, run serially; arch-lint clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (mapimport 85.2% against a floor of 80)

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

**Review Responses:** RR-0BE3QAY RR-1LRWBCR RR-3OHS70K RR-3PVTF04 RR-4FS0RDM RR-4XEJJM5 RR-53RWARY RR-5EO3W7B RR-82JYZL0 RR-8GKZ901 RR-AYR44ED RR-BKA3ORL RR-CKW2XTP RR-CLN5U82 RR-DE1HOQ8 RR-DXJ0URB RR-EKGKXHP RR-EM2WQF3 RR-EPZ1R65 RR-FRKI01E RR-GH9WPTH RR-IPF9VU5 RR-K0M9YSH RR-M8561E2 RR-N39KWOG RR-P2HPE9R RR-P3PFEJP RR-PDFVET9 RR-PUZYKTI RR-QLYXN2S RR-TS7PQD4 RR-TYLJ0HZ RR-URYJXPF RR-VSIHLAR RR-WSVZ1AR RR-YDZNU96 RR-YTK7GWN RR-ZNP2MZ2 RR-ZQVGHCH 

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
PASS for every criterion: stable keys, relations by key including same-run creates, missing reported never deleted, sandboxed hook, plan before apply (`--dry-run`, `-o json`), writes through entitymanager with PatchEntity, idempotent re-run, caps and no XXE, per-record errors with location, OSCAL example in tests and the guide. Evidence: IMPL-PSCQ9IY and the mapimport tests.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-9Q5TMO7

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh pr create after done; CI is not polled in this task)

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
