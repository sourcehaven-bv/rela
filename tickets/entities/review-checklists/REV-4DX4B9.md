---
id: REV-4DX4B9
type: review-checklist
title: 'Review: fsstore self-echo suppression is dead in production: SafeFS.OnPostWrite(RecordWrite) wiring was dropped with the encryption removal (#508)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — via `just ci`, 200 packages ok, race detector on.
- [x] Lint clean (`just lint`) — golangci-lint 0 issues (two rounds fixed a misspelling and a testifylint require-error).
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links; one `[postWriteObservable]` bracket dropped because Go cannot link an unexported type.
- [x] Coverage maintained (`just coverage-check`) — part of `just ci`, green.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — cranky-code-reviewer + rela-security-reviewer in parallel. Security: no findings (event path, echo LRU, git-crypt, fail-closed error all assessed; the single-slot hazard it flagged is the same as cranky C1).
- [x] All critical review-responses addressed — none raised.
- [x] All significant review-responses addressed — RR-AMRKMT (single observer slot → fan-out registry with remover, root-scoped recorder), RR-JYG7CA (fsnotify test pre-creates the leaf dir; verified failing on develop 3/3 in a throwaway worktree, passing 10/10 with -race on the branch).
- [x] Self-reviewed the diff for unrelated changes — code diff is 8 files under internal/storage, internal/store/fsstore, internal/app. The plimsoll-plan ticket files from the earlier session are NOT part of this PR's commit.

**Review Responses:** RR-AMRKMT, RR-JYG7CA (significant, addressed); RR-IMZGM5,
RR-L3682D (minor, addressed); RR-9MLJMO (nit, addressed); RR-0FNW41 (nit,
deferred — storetest hoist belongs to the fsstore arc).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) — see below.
- [x] Test evidence documented in implementation checklist — IMPL-RUUZ6S.

**Acceptance Status:**
- One Subscribe event and one observer EntityPut per store write with the watcher running, through the production FSFactory path: PASS (`TestFSFactoryWatcherSuppressesSelfEcho`; fails on develop with `[POL-1, POL-1, POL-2]`).
- Wiring cannot be forgotten by a construction site: PASS (`fsstore.New` installs it; `TestNewInstallsSelfEchoRecorder` uses no manual OnPostWrite).
- An FS without the capability fails closed at StartWatching, not silently: PASS (`TestStartWatchingRefusesUnobservableFS`).
- A second store on the same FS does not evict the first's recorder, and Close uninstalls: PASS (`TestTwoStoresOnOneFSKeepTheirOwnEchoes`, `TestSafeFS_OnPostWrite_FanOutAndRemove`).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing behaviour change)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

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
