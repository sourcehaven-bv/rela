---
id: BUGA-2FRANS
type: bug-analysis-checklist
title: 'Analysis: Conflict routes are not ACL-gated (content disclosure)'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (golden corpus `BUG-T6HNKS-conflict-hidden.viewer`: the viewer got 200 with hidden TKT-4's `secret` value; `BUG-T6HNKS-conflicts.*` listed TKT-4's path to every principal)
- [x] Minimal reproduction steps documented (GET /api/v1/_conflicts and GET /api/v1/_conflicts/entities/ticket/TKT-4.md as the viewer, on the golden fixture)
- [x] Environment/conditions noted (fs storage with a conflicted entity file; any policy; only fs storage produces merge conflicts)

## Root Cause

- [x] Immediate cause identified (why1: the conflict list and detail handlers call no permission check)
- [x] Contributing factors found (why2-3: the store cannot parse a conflicted file, so the routes read files directly and bypass the visibility readers; the routes predate git:sync)
- [x] Systemic cause explored (why4-5: file-reading routes outside the store have no default gate, and no guard lists them)

## Fix Planning

- [x] Fix approach determined (gate list, detail and resolve on git:sync via mayGitSync/permitsGitSync; resolve keeps its write check; row-gating rejected because conflicts exist only on local fs storage)
- [x] Regression test planned (TestV1Conflicts_GitSyncGate over every route and ACL mode; golden cases conflicts.* and conflict-hidden.*; SPA tests for the link and the 403 message)
- [x] Related areas checked for similar issues (GET /_git/status already hides conflict paths without git:sync; POST /_git/sync is gated; BUG-Q3Z15V duplicates this bug)
