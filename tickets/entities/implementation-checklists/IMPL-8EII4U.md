---
id: IMPL-8EII4U
type: implementation-checklist
title: 'Implementation: Sync 2/5: pgstore deletion tombstones + seq indexes + manifest query'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — internal/store/pgstore/tombstone_test.go (DB-gated)
- [x] Integration tests written — exercises real pgstore + listener (cross-process catch-up recovery)
- [x] Happy path implemented — migration 0003 (deletions table + seq indexes), tombstone writes in delete + rename paths, ManifestSince query, catch-up emits deletes
- [x] Edge cases handled — cascade delete tombstones each relation; rename tombstones old id + old relation triples; delete-then-recreate; tombstone seq ordering; missed-NOTIFY recovery via catch-up
- [x] Error handling in place — tombstone insert errors abort the delete/rename tx (atomic); manifest/catch-up errors wrapped/logged

## Test Quality

- [x] Using fixture builders/factories — newTombstoneStore/mustCreateEntity helpers; reuses newScopedPool/openWriter/freshFeedSchema
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object

## Manual Verification

- [x] Feature tested end-to-end — `go test -race -tags postgres ./internal/store/pgstore/` green against a live Postgres
- [x] Each acceptance criterion verified
- [x] Edge cases verified

**Verification Evidence:**
- `go test -race -tags postgres ./internal/store/pgstore/` → ok (against local Postgres). All build tags compile; dep gates: pgx in default=0, bleve in pgstore=0; arch-lint clean; my files lint clean.
- Migration 0003 applies from scratch → Status target=3; deletions table + 3 seq indexes exist.
- AC delete tombstone: TestDeleteWritesEntityTombstone / ...RelationTombstone — PASS.
- AC manifest changed+tombstones: TestManifestDeleteThenRecreate — PASS.
- AC seq indexes: TestSeqIndexesExist (existence; planner usage is scale-dependent, documented).
- AC missed-NOTIFY delete recovery (headline fix): TestCatchUpRecoversMissedDelete — PASS.
- AC cascade tombstones relations: TestCascadeDeleteTombstonesRelations — PASS.
- **Rename tombstones (code-review fix): TestRenameTombstonesOldIdentities — PASS.**

## Quality

- [x] Code follows project patterns — tombstone written in the same tx as the delete/rename (atomic with notify); manifest/catch-up share the UNION shape
- [x] Checked for DRY — writeTombstonesForEvents reuses the delete path's event slice; rename reuses writeEntity/RelationTombstone
- [x] No security issues — parameterized queries
- [x] No silent failures — tombstone insert errors abort the tx
- [x] No debug code left behind

## Code Review (cranky-code-reviewer)

Found 2 critical + 3 significant, all addressed:
- **RR-EOXQIB (crit)** — rename wrote no entity tombstone for oldID → ghost entity. FIXED: rename now tombstones oldID in-tx.
- **RR-ACJ0ZY (crit)** — rename re-keyed relations with no tombstone for old triples → ghost edges. FIXED: capture incident triples before re-key, tombstone each.
- **RR-AE54G9 (sig)** — unbounded deletions growth + cursor=0 replays history → documented retention caveat in ManifestSince godoc; pruning/pagination are follow-ups.
- **RR-QWC4OX (sig)** — non-CONCURRENT index build write-stall → documented maintenance-window requirement in 0003_sync.sql.
- **RR-947I7E (sig)** — stale overlap-window comment (cascade/rename now burn a seq block) → comment corrected.

The reviewer confirmed: tx atomicity correct, catch-up delete idempotency safe
(watcher collapses to type-scoped staleness), table-set invariant consistent,
attachments correctly excluded.

**Build-tag note:** pgstore uses IMPORT-based exclusion from default builds, not
per-file `//go:build postgres` tags. New files initially had the tag, which
broke the default build's type-check (untagged entity.go called the tagged
methods). Fixed by removing the tags. All three build tags compile.

## Follow-up tickets (deferred, documented)

- Tombstone retention/pruning + manifest pagination (LIMIT + next-cursor) — RR-AE54G9. Belongs with the sync API (TKT-PV0R3V) which consumes the manifest.
