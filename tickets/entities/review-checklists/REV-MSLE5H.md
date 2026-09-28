---
id: REV-MSLE5H
type: review-checklist
title: 'Review: Entity export 404s for every faced address, while the detail page still offers the Export menu'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `just coverage-check` ran the full suite; `go test -race ./internal/dataentry/` ok; frontend 3325/3325
- [x] Lint clean (`just lint`): 0 issues; frontend typecheck clean
- [x] Comment lint gate clean (`just comment-lint`): no new advisory findings in changed files
- [x] Coverage maintained (`just coverage-check`): PASS, total 80.6%

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
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-TZ4U5F, RR-WX0807)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-TZ4U5F, RR-WX0807, RR-BFSP6I, RR-BVIMSI, RR-0UMG10,
RR-MK29VP, RR-AIOW3O (addressed); RR-SZX2GP, RR-B2FZA0 (wont-fix, nits).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: `ID@face` exports that face, built-in and override
(`TestExport_FacedAddressExportsThatFace`,
`TestExport_FacedRenderOverrideReceivesTheAddress`).
2. PASS: an unreadable face is the missing-entity 404
(`TestExport_UnreadableFaceIsIndistinguishableFromMissing`).
3. PASS: the TKT-5SZG2L guard is gone; comments cite BUG-PLZDPR.

Anchored documents are fixed and face-gated (BUG-6DBV6N,
`TestDocument_FacedAnchorRendersThatFace`,
`TestFaceGrant_AnchoredDocumentIsFaceGated`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug; docs/transforms.md updated in the diff)
- [x] User-facing documentation updated (docs/transforms.md)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug, no docs-checklist)

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
