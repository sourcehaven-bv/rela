---
id: PLAN-34E1YZ
type: planning-checklist
title: 'Planning: Migration lock as a pluggable mini-service (postgres advisory lock, fs lock file)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- Consumer-side `MigrationLock` interface + `ErrLockHeld` in `internal/datamigration` (lock.go).
- Three implementations: pgstore session advisory lock (capability method on `*pgstore.Store`, discovered by type-assert — the ticket-sanctioned route), fs lock file under `.rela/` with pid-based stale detection, in-process mutex.
- `datamigration.LockFor(st store.Store, cacheDir string) MigrationLock` selector (build-agnostic: type-assert wins → fs lock file → process lock). No appbuild-only helper needed; both appbuild and CLI call LockFor.
- Consumers: `Runner.Run(apply=true)` acquires for the whole run (fail fast on contention); `GC.Tick(apply=true)` acquires (contention → `Skipped`, honest for both sweep and CLI); `GC.Scan` acquires (writes the ledger; contention → error); `Gate.Evaluate` adoption writes try the lock and on contention SKIP persisting (log, still publish the computed verdict) — non-blocking startup, and contention there means a runner is actively moving the marker, so skipping is correct.
- Deps: `Lock` REQUIRED (nil-rejected) in Runner `Deps` and `GCDeps` (destructive paths must not silently lose serialization); `NewGate` gains an optional lock parameter (nil = no lock, documented — gate-vs-gate races are content-identical).
- Wiring: appbuild `startDataMigration` and CLI `migrate_data.go` build the lock via `LockFor`.
- Docs: SaveMarker godoc updated (lock now exists); GUIDE-data-migration + GUIDE-postgres-backend concurrency paragraphs (docs are GENERATED — edit docs-project guides, run `just docs`).

NOT in scope:
- Marker CAS semantics (the lock makes the existing RMW safe).
- Generic distributed-lock service or any Tx-layer abstraction (DEC-8UIL0).
- Blocking waits/retries anywhere; only fail-fast TryAcquire.
- Sharing `sweepAdvisoryLockKey` — decision below.

**Acceptance Criteria:**

1. Two concurrent applies on one pg schema: second gets ErrLockHeld ("another migration or GC run is active"), zero writes. *DB-gated test: two `*pgstore.Store` on one schema, first holds, second TryMigrationLock ok=false; release frees.*
2. Schema isolation: two stores on DIFFERENT schemas of one database both acquire concurrently. *DB-gated test (the BUG-CA3VY0 regression class).*
3. GC apply and migration apply mutually exclusive (same lock). *Unit test with a held ProcessLock: Tick → Skipped, Runner apply → error.*
4. fs: second lock on the same `.rela/` fails fast; a lock file with a dead pid is detected stale, broken with a warning, and acquisition retried once; release removes the file; unparseable file treated stale. *Unit tests in a t.TempDir.*
5. Dry-runs and read paths never touch the lock. *Spy-lock test: Run(apply=false), Tick(apply=false) → zero TryAcquire calls.*
6. Gate contention: verdict published, marker NOT written, no error. *Unit test with held lock.*
7. LockFor selection: store implementing the capability → pg path (fake store in test); else cacheDir → fs lock; else process lock.
8. All existing datamigration/cli/appbuild tests updated and green; lint/arch-lint/plimsoll/coverage clean.

## Research

- [x] ~~/research doc~~ (skipped: approach fully specified in the ticket body, which was written from the TKT-0C57FS review analysis; this checklist refines the open decisions)
- [x] Checked codebase for existing patterns to reuse

**Existing Solutions:**

- `pgstore` already has the exact lock discipline to reuse: `tryAdvisoryLock`/`advisoryUnlock` (sweep.go:506-533, two-key schema-scoped form, cancellation-detached unlock) and the acquire-one-conn-hold-it pattern (sweep tick, purge). Key constants: "RELA" migrate, "RELD" reconcile, "RELV" sweep → new **"RELM" (0x52_45_4c_4d)** for the migration lock.
- Capability type-assert precedent: `store.Formatter`, `HistoryReader` — and the ticket explicitly allows it. This beats a recipe-supplied service here because the CLI (not just appbuild) constructs runners, and a type-assert keeps `LockFor` build-agnostic with zero new appbuild surface (Services is AT its 25-exported-method plimsoll cap).
- fs lock file: no existing helper; `os.OpenFile(O_CREATE|O_EXCL)` + pid liveness via `signal 0`. Single-machine by design (matches fsstore).
- Rejected: sharing `sweepAdvisoryLockKey`. A sweep tick holding the key for ~100ms would make `migrate data --apply` fail fast with a misleading "another migration is running"; the sweep-vs-migration interplay was already analyzed harmless in TKT-0C57FS (update captures dedup by content hash; deletes capture synchronously BEFORE removal). Separate key "RELM"; rationale recorded in the key's godoc.

## Approach

- [x] Technical approach chosen and documented
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:**

1. `internal/datamigration/lock.go`:
   - `type MigrationLock interface { TryAcquire(ctx) (release func(), err error) }`, `var ErrLockHeld`.
   - `storeLocker` (unexported consumer-side): `TryMigrationLock(ctx) (release func(), ok bool, err error)` — matches the pgstore method signature so no adapter package is needed; ok=false maps to ErrLockHeld.
   - `ProcessLock` (mutex TryLock), `fsLock` (lock file `.rela/migration.lock`, JSON `{pid, acquired_at}`; stale = pid not alive on this host OR unparseable; break stale with slog.Warn + one retry), `LockFor(st, cacheDir)` selector.
2. `internal/store/pgstore/migrationlock.go` (+ DB-gated test): `migrationAdvisoryLockKey = 0x52_45_4c_4d`; `(*Store).TryMigrationLock(ctx)`: `s.pool.Acquire` → `tryAdvisoryLock(conn, key)`; !ok → release conn, return ok=false; ok → release closure doing `advisoryUnlock(WithoutCancel(ctx), conn, key)` + `conn.Release()`. Session lock lives on the ONE held conn for its whole lifetime (sweep precedent).
3. Consumers: Runner.Run wraps the apply path; GC.Tick/Scan; Gate adoption (optional lock param). Contention behaviors per Scope.
4. Wiring: `startDataMigration` gains `cacheDir string` param; builds `lock := datamigration.LockFor(st, cacheDir)`, passes to gate + GC. CLI `migrate_data.go`: build once per command, pass to Runner/GC/gate.
5. Docs: SaveMarker godoc rewrite; docs-project guide paragraphs; regen `just docs`.

**Files:** new `internal/datamigration/lock.go` + `lock_test.go`,
`internal/store/pgstore/migrationlock.go` + `migrationlock_test.go`; modified
`internal/datamigration/{run,gc,gate,marker}.go` + tests,
`internal/appbuild/datamigration.go`, `internal/cli/migrate_data.go`,
`docs-project/entities/guides/GUIDE-{data-migration,postgres-backend}.md` (+
regenerated docs/), `.testcoverage.yml` untouched (floor already set).

## Security Considerations

- [x] Reviewed

- Lock file content is pid+timestamp JSON written 0644 under `.rela/` (gitignored cache); parsed defensively — unparseable = stale, never a crash. No secrets, no operator-controlled input on this path.
- Stale-break is same-host pid liveness only; it cannot be tricked into breaking a LIVE holder's lock by a crafted file (a file naming a live pid is honored; naming a dead/invalid pid means the holder is gone on a single-machine fs project).
- pg lock is schema-qualified (tenant isolation) and session-scoped on a held conn — no privilege or data exposure implications; failure to acquire only ever DENIES an operation.
- Fail direction everywhere: contention blocks destructive work, never permits it.

## Test Plan

- [x] Documented (mapped in Acceptance Criteria above)

Edge cases: release called twice (idempotent — guard with sync.Once); ctx
cancelled during acquire (error, conn released); fs lock dir missing (MkdirAll
first); lock file from THIS process's pid (treat as held — a second in-process
acquire must still fail via the embedded ProcessLock guard, so fsLock composes
pid-file + in-process mutex); GC sweep goroutine contention logs at debug via
existing Skipped plumbing.

Negative: Runner/GC constructors reject nil Lock; TryMigrationLock on closed
pool returns error (not held).

## Risk Assessment

- [x] Assessed. Effort m confirmed.

1. **Deadlock via forgotten release** → release is deferred at every acquire site; sync.Once makes double-release safe; pg conn release also frees the lock as a backstop.
2. **CLI UX regression** (gate now takes lock at startup) → gate uses skip-on-contention, never blocks/errs; only apply paths fail fast.
3. **pg pool exhaustion** (lock holds a conn for the whole run) → one conn per migration run, bounded and short-lived; same cost profile as a sweep tick; documented.
4. **Existing tests break on required Lock dep** → mechanical: tests pass NewProcessLock().

## Documentation Planning

- [x] SaveMarker godoc; GUIDE-data-migration (concurrency paragraph), GUIDE-postgres-backend (lock note in the data-migration section); no CLAUDE.md change needed (pattern already described there via state.KV precedent).

## Design Review

- [x] Inline design review performed (see below)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Three issues found and folded into the plan before
implementation: (1) *fs lock must compose an in-process mutex* — a pid-file
alone cannot exclude two goroutines in one process (same pid reads as "held by
me alive" for both); fsLock embeds ProcessLock first. Addressed in Test
Plan/approach. (2) *Gate contention must not fail startup* — original ticket
sketch said "short timeout"; refined to strict try-and-skip (no timeout knob, no
blocking) since a contended gate write means a runner owns the marker. Addressed
in Scope. (3) *Do NOT reuse sweepAdvisoryLockKey* despite the ticket suggesting
to consider it — a 100ms sweep tick would spuriously fail operator applies;
interplay already safe. Recorded with rationale in Research. No RR entities
needed (findings originated and were resolved within planning; the plan text is
the record).
