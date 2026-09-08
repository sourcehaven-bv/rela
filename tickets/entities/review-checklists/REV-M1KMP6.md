---
id: REV-M1KMP6
type: review-checklist
title: 'Review: config.Loader grows List and a disk-first layered loader'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race` over the affected packages; `just build-check-tags` clean)
- [x] Lint clean (`just lint`: 0 issues; `just arch-lint`: OK)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links)
- [x] Coverage maintained (internal/config 94.2% vs 87 floor)

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
- [x] All critical review-responses addressed (RR-RJ3QYT, RR-5932AS)
- [x] All significant review-responses addressed (RR-I7OQAW, RR-YASRJM, RR-5X9QPT)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-RJ3QYT, RR-5932AS (critical); RR-I7OQAW, RR-YASRJM,
RR-5X9QPT (significant); RR-L6VQKI, RR-YV817D (minor, one deferred to
TKT-S1EVV7).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: no implementation checklist; evidence is in the Acceptance Status block below)

**Acceptance Status:**

- PASS — `List` returns names sorted, relative to the prefix, non-recursive:
  `TestFSLoader_List` asserts the sorted set and that neither `nested` nor
  `deep.lua` appears.
- PASS — a missing directory lists empty rather than erroring:
  `TestFSLoader_List_MissingDirIsEmpty`. The neighbouring case (path exists
  but is a file) surfaces an error: `TestFSLoader_List_NotADirectoryIsAnError`.
- PASS — `NewLayered` falls back on `os.ErrNotExist` and only on that:
  `TestLayered_Load_FallsBackWhenAbsent` and
  `TestLayered_Load_NonNotExistErrorDoesNotFallBack`.
- PASS — traversal rejection for both new entry points:
  `TestFSLoader_List_RejectsUnsafeNames` (10 vectors) plus the pre-existing
  `TestFSLoader_Load_RejectsUnsafeNames`.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal seam, no user-facing surface yet)
- [x] ~~User-facing documentation updated~~ (N/A: `config.Loader` is internal; the user-facing `db dump`/`db load` commands land in Phase C)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist needed)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
