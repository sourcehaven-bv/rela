---
id: REV-ZKQZD2
type: review-checklist
title: 'Review: Index markdown id mentions as backlinks'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): every package passes. `internal/dataentry` exceeded the default 10m timeout at load average 40-50 in two full runs (a different test each time); rerun alone with `-race -cover -timeout 45m` it passes (1515s).
- [x] Lint clean (`just lint`): 0 issues; arch-lint and plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`); `comment-report` shows no finding introduced by this diff
- [x] Coverage maintained: every changed package is above its floor (visibility 89.7/85, output 94.2/90, dataentry 85.7/55, lua 86.3/80, mcp 78.7, cli 52.7, fsstore 84.5, sqlitestore 84.5, memstore 97.6, mentions 97.0)

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

- [x] Ran cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-UX4UO8, RR-E6DKTG, RR-62RY31, RR-8JFKGU)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-UX4UO8, RR-HUBIOD, RR-E6DKTG, RR-62RY31, RR-8JFKGU,
RR-EO08I2, RR-UXWZ2I, RR-80XH8N, RR-D5Y7DS (wont-fix, reason given), RR-R9YHP2,
RR-S4JJ6G, RR-9CAJPA, RR-8PVIR2, RR-J8JAWZ, RR-CQCSQH. Design-review RRs had
their resolutions rewritten to the as-built design.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: conformance CreateIndexesBody (mem, fs, sqlite, pg; cold and warm); TestMentionedBy_ListsReadableSources.
2. PASS: UpdateReplacesTargets.
3. PASS: DeleteFamilyDropsSource, SoftDeleteHidesAndUnmarkRestores.
4. PASS: RenameSourceMovesIt, RenameTargetLeavesTextAlone.
5. PASS: TargetNeedNotExist.
6. PASS: FacesIndexedSeparately, TestResolver_MentionedBy faced cases, TestMentionedBy_FacedSourceFollowsTheWorld, forward faced-link tests.
7. PASS: CodeBlocksAndLinksDoNotCount, TestTargets; manual fenced-block check.
8. PASS: TestResolver_MentionedBy gate cases, TestResolver_MentionedByCursorNamesNoHiddenFace, TestMentionedBy_HiddenTargetIs404, RedactsHiddenTitle, MCP TestACL_ShowEntity_WithholdsHiddenNeighbor, Lua TestScriptReads_MentionedByGated.
9. PASS: pgstore TestMentions_VisibleAcrossStores.
10. PASS: fsstore TestMentions_ExternalEditUpdatesIndex, StaleCacheIsReread; manual external edit.
11. PASS: sqlite TestMentions_UnindexedRowIsIndexedOnRead, pg TestMentions_OlderWriterHealed, fs lazy build.
12. PASS: REST handler tests, Vitest panel tests, e2e mentioned-by.spec.ts (6/6 with entity-refs), CLI TestShowEntityMentionedBy and TestWriteEntityMentionedBy, MCP TestHandleShowEntity_MentionedBy, Lua tests.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-6P5634

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: the PR is opened after the ticket is done, on request)

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
