---
id: REV-199HT
type: review-checklist
title: 'Review: Migrate fsstore write paths to RootedFS (closes CodeQL path-injection alerts)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — fsstore, storage, workspace, storetest all green
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — 72.8% total (up from 72.6% on develop)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] ~~All critical review-responses addressed~~ (N/A: the one critical flag turned out to be pre-existing behavior, verified against origin/develop)
- [x] All significant review-responses addressed — 6 significant findings: 2 addressed (RR-AXH84, RR-F7TFV), 4 deferred/wont-fix with documented reasons
- [x] Self-reviewed the diff for unrelated changes — tight scope: 3 new storage methods (OpenForWrite, AbsPath, SupportsStreaming), fsstore fields renamed to *Key, all data I/O routed through *RootedFS, factory + tests updated

**Review Responses:** 15 review-responses created.

- **Significant (7)**: RR-C45OR (deferred — streamToFile atomicity pre-existing), RR-0QKRG (deferred — decorator extensibility, no second decorator exists), RR-AXH84 (addressed — absPath no longer panics), RR-5F3KE (wont-fix — speculative defense against future change), RR-F7TFV (addressed — sanitizer test rewrite), RR-69TXX (wont-fix — Windows canonical casing, no Windows CI), RR-IL1WV (deferred — factory doesn't wire OnPostWrite, pre-existing bug).
- **Minor (4)**: RR-5X07F (addressed — EntitiesKey/RelationsKey empty check), RR-DLP50 (addressed — slog.Warn on cleanup walk failure), RR-KH3O0 (wont-fix — forward-compat cache format), RR-BZRE5 (addressed — cached SupportsStreaming).
- **Nits (4)**: RR-NDRK4, RR-PW4AO, RR-SA3RV, RR-065O2 — all deferred/wont-fix with documented reasons.

No critical or significant findings left open.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1** (Config.Rooted required, fsstore.rooted field): PASS — New() rejects nil Rooted + empty EntitiesKey + empty RelationsKey. All existing conformance/persistence/recovery/differential tests pass.
- **AC2** (5 write sinks go through *RootedFS): PASS — writeDataFile, saveIndex, writeAttachment (buffered MemFS branch), streamToFile (OsFS via OpenForWrite), and all data-file Removes now flow through s.rooted. Verified by diff audit.
- **AC3** (data-file Removes migrated): PASS — entity.go + relation.go use s.rooted.Remove(key). Attachment Removes also migrated (Option 2 gave us keys throughout, so no raw s.dirs.Remove remains anywhere in the package).
- **AC4** (OpenForWrite exists): PASS — TestRootedFS_OpenForWrite_* (3 tests) + TestRootedFS_AbsPath* (2 tests). resolve() stays unexported; AbsPath is the narrow public accessor for the watcher/LRU use case.
- **AC5** (sanitizer agreement): PASS — TestUploadSanitizerAgreesWithRootedFS exercises the real filepath.Base + rfs.WriteFile path. Inputs where Base strips bad bits (path traversal) succeed; inputs Base preserves but RootedFS rejects (CON.txt, file:name.txt, ..) fail loud. No silent corruption.
- **AC6** (CodeQL alerts close): verify post-merge.
- **AC7** (CI green): local `just lint` → 0 issues, `just test` → all PASS, `just coverage-check` → PASS (72.8%).

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via has-docs~~ (N/A: internal refactor)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

Package-level doc comment in fsstore.go documents the rooted/rawReader split.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** *(pending)*
