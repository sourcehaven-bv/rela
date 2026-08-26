---
id: DOCS-6JHU6K
type: docs-checklist
title: 'Docs: Migration lock mini-service (TKT-CPCBR7)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `MigrationLock`/`ErrLockHeld`/`LockFor` (selection rationale incl. why memstore-with-project gets the fs lock), `ProcessLock` (generation-guarded releases), `fsLock` (break-mutex TOCTOU design, pid-reuse limitation + remedy), `(*pgstore.Store).TryMigrationLock` (pinned-connection rule, pool preflight), `migrationAdvisoryLockKey` (why NOT sweepAdvisoryLockKey)
- [x] `SaveMarker` godoc rewritten: the lock is now the safety mechanism; residual two-gates race documented
- [x] pgstore plimsoll directive bump justified in the type comment

## Project Documentation

- [x] `docs-project/entities/guides/GUIDE-data-migration.md` — concurrency paragraph (fail-fast apply, sweep skip, gate skip, dry-runs lock-free) + crash-recovery caveat (pid reuse, manual `rm .rela/migration.lock` remedy); regenerated into docs/
- [x] `docs-project/entities/guides/GUIDE-postgres-backend.md` — schema-scoped advisory migration lock bullet; regenerated
- [x] No CLAUDE.md change needed (capability-selection pattern already documented via the state.KV precedent)

## External Docs

- [x] ~~Website/blog~~ (N/A: docs/ is the published surface)
