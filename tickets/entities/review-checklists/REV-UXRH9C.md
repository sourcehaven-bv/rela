---
id: REV-UXRH9C
type: review-checklist
title: 'Review: History, restore and history-purge ignore faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7VER8N (critical), RR-EEFDVG (critical), RR-50YAJ2, RR-XXQGNG, RR-NQJRZM, RR-C47YN9, RR-CUGX5F (significant), RR-YGWX0O, RR-4N3YP2, RR-L3TQTC, RR-Y0AD4M, RR-7WH2T1 (minor, addressed), RR-SGJVSA (minor, deferred: audit subjects carry no face project-wide)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- HTTP history/restore take `ID@face` through the resolver: PASS (history_face_test.go, history_face_guard_test.go).
- CLI history/restore/history-purge/relation-history* parse `ID@face`; a bare faced id errors naming the faces: PASS (history_address_test.go).
- Purge reaches one face or tail; `--all` is the fenced lineage of one face: PASS (storetest PurgeByContentHashIsScopedToOneFace, RelationPurgeIsScopedToOneTail, RelationRecordIDIsBoundToItsTail on pg and sqlite).
- Restoring a deleted face recreates it at its face, create-only: PASS (TestFacedHistory_RestoreRecreatesADeletedFace, TestFacedHistory_RecreateNeverOverwritesALiveFace).
- e2e faces-history.spec.ts un-fixme'd: PASS on postgres (42 faces and history specs).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug; docs updated under TKT-7R0ABK's DOCS-0IU91C)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened after done against faces-intrinsic, as part of TKT-2528AB PR 6)

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
