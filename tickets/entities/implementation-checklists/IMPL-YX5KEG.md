---
id: IMPL-YX5KEG
type: implementation-checklist
title: 'Implementation: Retire the second GET channel: make the sync feed content-free + row-gated, fetch through the authorized read path'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (mergeProperties splice; two-token index + legacy migration)
- [x] Integration tests written (full client↔fakeServer /api/v1 round-trips: create/patch/delete, pull-splice, conflict)
- [x] Happy path implemented (pull via /api/v1 with _redacted splice; push via /api/v1 with temp-id adoption)
- [x] Edge cases from planning handled (redacted≠deleted splice; temp-id relation remap; feed-404 hidden-not-deleted; schema-compat handshake)
- [x] Error handling in place (fail-closed relation read; schema-drift fail-fast; mid-batch resume; conflict halts one record)

## Test Quality

- [x] Table-driven splice tests covering all three property states (present/hidden/deleted)
- [x] No hardcoded values in assertions when object is in scope (minted ids discovered via onlyServerEntityID)
- [x] Only specifying values that matter (granted vs hidden field per test)
- [x] ~~Interpolated values constructed from objects~~ (N/A: fixed seed ids read clearer here)
- [x] Property comparisons assert the visible value updates AND the hidden value survives locally

## Manual Verification

- [x] ~~Feature manually tested end-to-end via UI~~ (N/A: machine-to-machine sync channel, no UI; covered by client+handler tests)
- [x] Each acceptance criterion verified with a test scenario
- [x] Edge cases verified (temp-id adopt+remap, feed-404, both-dirty conflict, mid-batch resume)

**Verification Evidence (fancy-browser model, TKT-8P1TM7):**

- Splice crux: `TestMergeProperties` (internal/cli/sync) — hidden preserved, deleted dropped, visible upserted; the redacted≠deleted guarantee.
- End-to-end redaction: `TestPull_RedactedField_PreservesLocalHiddenValue` — a redacted pull updates the visible field and PRESERVES the local hidden value through the real client + fakeServer.
- Temp-id + relations: `TestPush_CreateUpdateDelete_Converges`, `TestPush_TopologicalOrder_EntitiesBeforeRelations` — minted-id create, local rename, relation endpoint remap in one pass (RR-SYNCR2).
- Conflict + resume: `TestPush_Conflict_HaltsThenForceResolves`, `TestPush_CreateConflict409_...`, `TestPush_MidBatchFailure_ResumesOnRerun`, `TestPull_BothDirty_Conflict`.
- Feed-404 guard (RR-SYNCR3): applyOne skips (never deletes) on a bare 404 for a non-tombstone feed entry.
- Server relation read (RR-SYNCR1): handleV1GetRelationTarget — body + relation ETag, dual-endpoint gated, fail-closed meta.
- Index migration: `TestLoadState_MigratesLegacySingleStringFormat`.
- Local CI green: golangci-lint 0 issues, `internal/cli/sync` + `internal/dataentry` suites pass (only the pre-existing gitignored-CSS-build test fails, unrelated), default + postgres builds, arch-lint clean, plimsoll (god-object) passes.

## Quality

- [x] Code follows project patterns (consumer-side interfaces; reuses ApplyEntity/RenameEntity; package-fn handler to respect god-object line)
- [x] Checked for DRY — one splice implementation (mergeProperties) drives entity + relation pull; redaction inherited from the single /api/v1 read path, not re-implemented
- [x] No security issues introduced (this IS a security fix — closes the read-redaction AND write-field-ACL bypass by retiring the parallel channel; splice can't erase hidden fields; push inherits validateFieldWrite)
- [x] No silent failures (schema drift fails fast; source-gone → empty meta; bare-404 → skip not delete, all documented)
- [x] No debug code left behind
