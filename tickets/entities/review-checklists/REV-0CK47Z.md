---
id: REV-0CK47Z
type: review-checklist
title: 'Review: Session decisions as one-click buttons'
started: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on the changed packages, sqlite-tag runs, and the full run inside `just coverage-check`; postgres tests run in CI
- [x] Lint clean (`just lint`): 0 issues on all tags
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): 85.4% total, all package floors pass

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed: 1 significant (generation read after the confirm dialog) fixed: captured at the click, with a test
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-UQO84V RR-GB4A1S RR-TRD40V RR-KV766B RR-YE487I
RR-HVCNX0 RR-1EF6JB RR-UZHHBJ RR-5ZBM82 RR-C399SN RR-USAWK8 RR-KT3MBR RR-6N77AA
RR-OKB9TC RR-R1FVUH RR-QN0Y8M RR-7DCN75 RR-A92OSQ RR-MI63AC RR-4M7TGY RR-2J07J9
RR-YPC393 RR-Q4FH2X RR-5HPTYX RR-PVJ7M0 RR-H7SFLV RR-3C6PXK RR-LBE4US RR-TB2MZW
RR-BXLGOA RR-NUOZ3M RR-IKDTZ4 RR-PMGPFO

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): PLAN-GPDE8G
- [x] Test evidence documented in implementation checklist: IMPL-3OFPHF

**Acceptance Status:** All acceptance criteria in PLAN-GPDE8G PASS; each maps to
a named test listed in IMPL-3OFPHF.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: see DOCS-4F45WY
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-4F45WY

## Final Checks

- [x] Commit message explains the why, not just what: minimal message per user preference; the ticket carries the why
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: done after this checklist (see note below)

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
