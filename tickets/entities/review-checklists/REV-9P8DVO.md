---
id: REV-9P8DVO
type: review-checklist
title: 'Review: Root-base guard misses the Windows drive root'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

`just arch-lint`, `just plimsoll` and `just lint-md` also clean.

One comment-lint finding was raised by this diff and FIXED rather than
suppressed: the godoc wrote `[filepath.Clean]ed`, and the trailing suffix makes
the doc link unresolvable so godoc renders the brackets literally. Reworded to
"an absolute p passed through [filepath.Clean]". The rule was right; suppressing
it would have shipped visibly broken godoc on the one comment this change exists
to leave behind.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0DTTMS (critical, addressed), RR-S2X70O (significant,
addressed), RR-T1XVAX (minor, addressed).

The critical finding was that the first implementation still failed open for UNC
shares: `filepath.Clean` returns `\\server\share` unchanged (the whole string is
the volume name, so there is no remainder to root and no separator is appended),
and the predicate only matched `volume+separator`. `filepath.Rel` then treats
that bare base as absolute — its own source comment says so — so every path on
the share was reported contained. Fixed with a second, volume-guarded clause.

Worth stating plainly: the test table had ASSERTED the wrong `Clean` output, so
it passed green over a string Windows never produces while the string it does
produce went unguarded. That is the same defect shape as the ticket itself,
reproduced one level up in the tests. RR-S2X70O is the structural answer.

Diff self-review: three files, all in `internal/git`. No unrelated changes. The
`tickets/` entities are workflow bookkeeping, not product code.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 (Windows drive root refused) — PASS. `windows drive root`,
`windows drive root lowercase`, `extended drive volume`, `extended drive root`.
- AC2 (UNC share root refused) — PASS, after correction. BOTH
`unc share root trailing separator` and `unc share root bare volume`; the latter
is the spelling that actually occurs and was added in review.
- AC3 (Unix refusal preserved by the same predicate) — PASS. Row `unix root`
plus the unmodified `TestContainedPath_RejectsRootBase`.
- AC4 (no over-refusal) — PASS. Six negative rows, including `windows drive
relative` and `empty path no volume`, plus the unmodified
`TestClone_AllowsPathInsideBaseDir`.
- AC5 (existing behaviour unchanged) — PASS. Full `internal/git` suite green with
`-count=1`; no pre-existing test was edited.

Mutation evidence for all of the above is in IMPL-33H5RI: four mutations, each
confirmed to have landed on a numbered line of executable code before running,
each reddening a specific and expected subset.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] ~~User-facing documentation updated~~ (N/A: no user-observable change —
`internal/git` has no published surface, no command or flag changes, and the one
in-tree caller derives its base from `os.UserHomeDir`, which is never a volume
root on any platform. Full reasoning in DOCS-ZFWTD6)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZFWTD6

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
