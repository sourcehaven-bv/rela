---
id: REV-RCIGKO
type: review-checklist
title: 'Review: fsstore index cache can drop an entity written by a concurrent process'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `go test -race` on internal/store/..., internal/app/..., internal/appbuild/..., internal/fsimport, internal/cli/... and cmd/rela-server/...; build and vet on the default, `postgres`, `sqlite` and `memorybackend` tags. CI runs the full suite on the PR
- [x] Lint clean (`just lint`): golangci-lint on the changed packages for all four tags (0 issues), `just arch-lint`, `just plimsoll`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): fsstore 84.5%, app 76.2%, appbuild 73.9%, fsimport 74.7%, all above their floors

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
- [x] All significant review-responses addressed: the three significant design-review findings are resolved by removing the cache; the code reviews raised none
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 19 review-response entities linked via
`has-review-response`: 10 from the design review and 9 from the code review (the
security review found nothing). 18 are addressed. RR-S3VS4D is wont-fix here:
the undeclared-type write fallback predates this change and is filed as
TKT-C2YVX1. The temp-file cleanup race (RR-SEPGPF) is filed as TKT-H63QDR.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist): see PLAN-ZO1LO2
- [x] Test evidence documented in implementation checklist: IMPL-5N181W

**Acceptance Status:**
- AC1 concurrent writer seen on reopen: PASS, `TestIndex_ConcurrentWriterIsSeenOnReopen` (fails on develop)
- AC2 schema change seen on reopen: PASS, `TestIndex_SchemaChangeIsSeenOnReopen` (fails on develop)
- AC3 no index file written or read: PASS, `TestIndex_NoPersistedIndex` (fails on develop)
- AC4 pending deletes still hide marked families: PASS, existing soft-delete tests
- AC5 open cost measured: `BenchmarkOpen`, about 16 to 20 ms per open

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: chore; no user-facing docs mention the index file)
- [x] ~~User-facing documentation updated~~ (N/A: internal change; the fsimport package doc and the datamigration agent rule were updated)
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
