---
id: REV-HGE4KW
type: review-checklist
title: 'Review: Deleted-relation history id-reuse disambiguation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (pgstore postgres suite + dataentry + cli)
- [x] Lint clean (default build); postgres test file not linted in CI
- [x] ~~Coverage maintained~~ (N/A: adds tests)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer)
- [x] All critical review-responses addressed (none — no critical findings)
- [x] All significant review-responses addressed (RR-HGEP1 HTTP parse-swallow, RR-HGEP2 store-side mutual exclusion, RR-HGEP3 missing rename tests)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-HGEP1, RR-HGEP2, RR-HGEP3 (all addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented

**Acceptance Status:**

- Enumerate all lifetimes of a reused key: PASS (`TestListRelationLifetimes_ReusedKeyEnumeratesAll` — older lifetime reachable, was orphaned before).
- Address a specific lifetime, auth-bounded: PASS (`TestRelationHistoryQuery_RecordIDMustBelongToKey`).
- Rename fold / renamed-away exclusion: PASS (`TestListRelationLifetimes_ForkedRenameStitchesToOneLifetime`, `_RenamedAwayNotListedUnderOldKey`).
- Purge compliance (refuse multi-lifetime; per-lifetime; all-lifetimes; mutual exclusion): PASS (`TestPurgeRelationVersions_*`).
- HTTP `_lifetimes` + `record_id` (auth, 400 on malformed): PASS (`TestRelationHistory_LifetimesRoute*`, `_BadRecordIDIs400`).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] Developer docs updated (CLAUDE.md unchanged — mechanism documented in godoc + design)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HGE4KW

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (monitored post-creation)
- [x] PR URL documented below

**PR:** (filled after creation)
