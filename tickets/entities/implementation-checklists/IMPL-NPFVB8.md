---
id: IMPL-NPFVB8
type: implementation-checklist
title: 'Implementation: Data migration system: shape hash, compatibility classifier, declarative migrations, GC sweep'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented per the plan (PLAN-OX2A9U)
- [x] Edge cases from planning handled

**What was built** (branch `tkt-0c57fs-data-migration`, commits
41459bf7..df7f2182):

- `internal/metamodel/shapeprojection.go` + `shapecompare.go`: `ShapeProjection` (data-shape fingerprint, id prefixes excluded per A5), `Hash()` (length-prefixed key-sorted SHA-256, tag 'S'), `CompareShapes` three-tier classifier with possible-rename pair detection, structured deltas (Counterpart/Removed/Added) for the generator.
- `internal/datamigration/`: marker + drift ledger in state.KV; Gate (bootstrap/in-sync/adopt/needs-migration, verdicts via atomic.Pointer); migration file format with embedded from/to projections integrity-checked against hashes (A2); chain resolver with compatibility as free edges; 10 idempotent steps incl. pure-transform sandboxed Lua (no io/os, engine applies the returned patch); Runner (dry-run default, Tx-batched raw writes, marker advance per file, one audit record per file, synchronous pre-delete version capture via VersionCapture per A1); Generator (GUESS/TODO drafts, deletions only commented, drafts round-trip the parser); GC engine (schema-name-keyed ledger per A6, grace period default 30d, skips while needs-migration, audit as constructor dep per A8, `Scan` for legacy orphans).
- `internal/store/fsstore` fix (A3): `updateEntity` relocates the file on type change; pinned by storetest `UpdateChangesType` (all backends) + fsstore `TestPersistence_TypeChangeLeavesNoOrphanFile` (verified red without the fix, green with — the reopen-only version was masked by MemFS scan order, so the test asserts file layout directly).
- `internal/cli`: `rela migrate status|gen|data|gc` (kong subcommands; bare `rela migrate` stays service-free via full-command-path matching in `requiresProject`); `gc --grace/--scan/--apply`; `status` exits 1 while unmigrated (CI-usable).
- `internal/appbuild/datamigration.go`: gate evaluated per assemble; GC sweep goroutine (RELA_DATA_GC=off, `_INTERVAL`, `_GRACE`), stopped in Services.Close.
- `internal/audit`: `OpDataMigration`, `OpDataGC` ops.
- Docs: new `docs/data-migration.md`; cli-reference, postgres-backend, metamodel sections; CLAUDE.md rule (two hashes, third sanctioned raw-write exception); arch-lint component + rules.

**Deviations from plan** (all recorded on the RRs):
- A4 refined: no server-side metamodel hot-reload exists today (rebuildState fires only for data-entry.yaml), so the gate evaluates per process start; the reload hook is one documented call when live reload lands (RR-FURO8P resolution updated).
- No dedicated pg advisory lock for the migration run in v1: writes serialize through `store.Tx` per batch (per-backend contract); sweep interplay is capture-noise-only for updates (dedup by content hash) and deletes capture synchronously before removal. The purge-style lock exclusivity concern applied to erasure, which migration is not.
- Enum-value drift is NOT GC'd (out-of-scope note: values are `map_values` territory, not orphan cleanup) — ledger records only property/type orphans.

## Manual Verification (evidence)

End-to-end on a scratch fs project (/tmp/tkt-e2e, rela built from branch):

1. Fresh project → `migrate status`: "baseline adopted", then "in sync" (AC2 bootstrap). ✓
2. Edited schema.yaml: `status`→`state` rename (delete+add), enum `open/wip`→`todo/doing`, `due` string→date. Gate blocked with per-delta warnings incl. the possible-rename notice; `status` exit 1 (AC3/AC4). ✓
3. `migrate gen` wrote `0001-schema-change.yaml`: rename_property GUESS, convert GUESS, map_values TODO with positional pairs (open→todo, wip→doing), embedded projections (AC5). ✓
4. `migrate data` dry-run: per-step counts (2/1/0 — map_values correctly 0 because the rename hadn't been applied yet) + validation delta line; no writes, marker untouched (AC6). ✓
5. `--apply`: entity files show `state: todo`, `due: "2026-01-02"`; validation 1→0; marker advanced with applied list; audit record `data-migration` with counts, no content; re-run → clean no-op "in sync" (AC6). ✓
6. Deleted `due` from schema → drift-adopted; `migrate gc` shows pending with 2026-09-19 deadline; `--grace 1s --apply` after 2s deleted the value and wrote a `data-gc` audit record (AC9, grace + audit). ✓

## Quality

- [x] `just lint` — 0 issues
- [x] `just arch-lint` — clean (datamigration component added, gopher-lua vendor allowance documented)
- [x] `just plimsoll` — clean (Metamodel pin 31→32 for ShapeProjection, justified at the directive)
- [x] `go test ./...` — all green (incl. store conformance suites); `just coverage-check` PASS (datamigration 61.5% vs 50 floor; total 77.2%)
- [x] No silent failures: gate/GC degrade with logged warnings, never swallow; step errors abort before the marker is written

**Not run here:** the pg-gated suite (`RELA_TEST_DATABASE_URL` not set locally)
— the postgres CI job covers the storetest conformance case on pgstore;
version-capture pg paths exercised via the VersionCapture seam in unit tests.
