---
id: IMPL-68H55Q
type: implementation-checklist
title: 'Implementation: Migration lock as a pluggable mini-service (postgres advisory lock, fs lock file)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented per PLAN-34E1YZ
- [x] Edge cases from planning handled

**What was built** (branch `tkt-cpcbr7-migration-lock`, commit 3222c401):

- `internal/datamigration/lock.go`: consumer-side `MigrationLock` + `ErrLockHeld`; `ProcessLock` (mutex TryLock); `fsLock` (`.rela/migration.lock`, O_CREATE|O_EXCL, pid+timestamp JSON, stale-break only on dead/unattributable pid — never age; composes the process mutex per design-review finding 1); `LockFor` selector (store capability type-assert → fs lock file → process lock); idempotent releases via sync.Once.
- `internal/store/pgstore/migrationlock.go`: `(*Store).TryMigrationLock` — new key "RELM" (0x52_45_4c_4d), two-key schema-qualified form, session lock pinned to ONE held pool conn (sweep-tick rule), detached-ctx unlock, release sync.Once; NOT sharing sweepAdvisoryLockKey (rationale in godoc per planning decision 3). Plimsoll pin 38/48 → 39/49 with row-property justification at the directive.
- Consumers: `Runner.Run(apply)` holds the lock for the whole run (fail fast); `GC.Tick(apply)` skip-on-held via `Skipped`; `GC.Scan` acquires (writes the ledger); `Gate` takes an optional lock and SKIPS persisting adoption on contention (verdict still published) — startup never blocks (design-review finding 2). Dry-runs/read paths lock-free. `Lock` nil-rejected in Runner Deps and GCDeps.
- Wiring: `appbuild.startDataMigration` builds one lock per assembled store via `LockFor(st, cfg.Paths.CacheDir)`, shared by gate + GC sweep; CLI `migrate_data.go` builds the equivalent lock per command — the two exclude each other because LockFor derives from the same store/cache dir.
- Docs: SaveMarker godoc rewritten (lock is now the safety mechanism; two-gates race stays content-identical); GUIDE-data-migration concurrency paragraph + GUIDE-postgres-backend advisory-lock bullet, regenerated via `just docs`.

## Manual Verification (evidence)

Scratch fs project (/tmp/lk-e2e, rela built from branch):

1. Pending needs-migration change; wrote `.rela/migration.lock` naming a LIVE pid (background sleep) → `migrate data --apply` failed fast with "another migration or GC run is active", **exit 1**, zero writes. ✓
2. Killed the holder → next apply logged the stale-break warning, broke the lock, applied the migration, and the lock file was gone after release. ✓
3. Normal flow (gen → apply → in sync) unchanged. ✓

## Quality

- [x] `just lint` — 0 issues; `just arch-lint` clean; `just plimsoll` clean (pgstore pin bumped with justification)
- [x] `go test ./...` all green (default build); DB-gated `TestMigrationLock_ExclusiveWithinSchema` + `TestMigrationLock_SchemasAreIndependent` PASS with -race against local postgres (AC1 + AC2/BUG-CA3VY0 class)
- [x] datamigration coverage 65.4% (floor 50); `just docs-check` passes once committed (regeneration is clean)
- [x] Unit tests: process/fs exclusivity, stale/unparseable/zero-pid break, live-pid honored, LockFor selection (3 paths), dry-runs never touch the lock (spy), contended apply → ErrLockHeld with zero writes, GC apply skip + Scan error on contention, gate skip-and-publish + persist-after-release, nil-Lock constructor rejection, double release everywhere.
