---
id: REV-E96MVE
type: review-checklist
title: 'Review: Sync 5/5: rela CLI sync client — index, topo-ordered diff, push/pull, manual --force'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./internal/cli/...`)
- [x] Lint clean (`golangci-lint run ./internal/cli/...`)
- [x] Coverage maintained (sync pkg 71%, above the 50 floor; `just coverage-check` green)

## Code Review

- [x] Ran cranky-code-reviewer on the full diff
- [x] All critical review-responses addressed (RR-1CZUZB, RR-56C2KY)
- [x] All significant review-responses addressed (RR-FVMO2Y)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-1CZUZB (critical, addressed), RR-56C2KY (critical,
addressed), RR-FVMO2Y (significant, addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC#1 converge (create/update/delete push) — PASS (TestPush_CreateUpdateDelete_Converges)
- AC#2 mirror (remote create/update/delete pull, tombstone→local delete) — PASS (TestPull_RemoteChanges_Mirror)
- AC#3 conflict halt + --force resolves + re-baseline — PASS (TestPush_Conflict_HaltsThenForceResolves, TestPull_BothDirty_Conflict)
- AC#6 topo order (relation before endpoint reordered) — PASS (TestPush_TopologicalOrder_EntitiesBeforeRelations)
- Idempotent replay / mid-batch resume — PASS (TestPush_MidBatchFailure_ResumesOnRerun, TestPull_RelationTombstone_IdempotentOnResume)
- --force unknown id → clear error, no partial state — PASS (TestForcePush_UnknownRecord_Errors)
- Bearer auth, 403 distinct from 412/422, token never logged — PASS (TestAuth_BearerToken)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-VWM0Y8)
- [x] User-facing documentation updated (docs/sync.md, docs/cli-reference.md)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VWM0Y8

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created and merged (#1027 → sync-pg-tombstones, the FEAT-NJ9FEN umbrella)
- [x] All CI checks pass (auto-merged)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1027 (merged into
sync-pg-tombstones / #1010; reaches develop when the umbrella merges)
