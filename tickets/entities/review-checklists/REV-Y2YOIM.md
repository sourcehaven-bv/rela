---
id: REV-Y2YOIM
type: review-checklist
title: 'Review: Data-migration face move and delete leave comment threads behind'
started: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (every package passes except cmd/rela-desktop TestChromeStyle_TargetsShippedClasses, which reads the locally built SPA in internal/dataentry/static, stale since 2026-09-26; this diff touches neither the desktop app nor the frontend)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (go-test-coverage on the run's profile: package and total floors pass, 82.8% total)

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

**Review Responses:** RR-LUSKOZ, RR-C9UH50, RR-VZ2MX2, RR-6SKLU7, RR-VHA467
(significant, addressed); RR-DV1W43, RR-IFDCZY, RR-XH9DHI, RR-G0ZJCY, RR-5JF80H
(minor, addressed); RR-FQBZ3M, RR-5AUXXX (minor, deferred; the latter is
BUG-B9P9AT); RR-CA19MV, RR-HJGB5H, RR-G33Q71 (nit, addressed); RR-ALM2WD (nit,
wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Face move (rename_face, migrate_face, adopt-face) moves the thread: PASS (datamigration/comments_test.go, cli TestMigrateAdoptFace_MovesCommentThreads).
- drop_entities and the GC sweep drop threads: PASS (TestDropEntities_DropsCommentThreads, TestGC_DropsCommentThreadsOfCollectedEntities, cli TestMigrateData_DropEntitiesDropsCommentThreads, TestMigrateGC_DropsCommentThreads).
- Dry-run leaves threads; a failure converges on re-run; a collision moves nothing: PASS (dry-run, RerunConverges, Collision tests).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; CLAUDE.md raw-store note updated)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: runs after done, on the user's go-ahead)

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
