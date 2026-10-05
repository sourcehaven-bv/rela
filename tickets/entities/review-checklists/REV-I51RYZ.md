---
id: REV-I51RYZ
type: review-checklist
title: 'Review: Rename authorizes the zero face but re-keys every face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (via `just coverage-check`; the rename and delete tests also ran on sqlite and postgres)
- [x] Lint clean (`just lint`) (golangci-lint 0 issues; arch-lint, plimsoll and lint-md clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky and rela-security reviewers)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-SCJGXW)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-SCJGXW, RR-CHKZ65, RR-WW9P9A, RR-SRIC1I, RR-XUW1JB,
RR-ICB0MX, RR-JU1MPS, RR-8RADMD, RR-2NNKG6, RR-EL2BK7, RR-3COZCN

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (fix plan in BUGA-M40677)
- [x] Test evidence documented in implementation checklist (IMPL-DVDKPN)

**Acceptance Status:**

- PASS: type-wide `update: [policy]`, draft-only and published-only principals cannot rename a faced family, and nothing is written (TestFamilyRename_DeniedUnlessEveryFaceIsRenamable).
- PASS: update on every face renames all faces with one rename version and one audit record per face (TestFamilyRename_EveryFaceCapturedAndAudited).
- PASS: a face that appears mid-operation is authorized (TestFamilyRename_FaceAddedDuringRenameIsAuthorized).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (rename paragraph in the ACL security guide)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR against faces-intrinsic, opened after done)

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
