---
id: PLAN-ZO1LO2
type: planning-checklist
title: 'Planning: fsstore index cache can drop an entity written by a concurrent process'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** fsstore keeps its index cache in `fsstore-index.json`. On open,
`syncIndex` restores the entity index from the cache when the newest entity type
directory mtime equals the cached one, and the relation index likewise
(internal/store/fsstore/index.go). The cache is written on `Close` (watcher.go),
and `savePersistedIndex` reads the directory mtimes **at save time**. The
in-memory index was built at open time. So:

1. Process A opens and reads the directories.
2. Process B creates `entities/plans/PLAN-X.md`; the `plans` directory mtime
changes.
3. A closes and saves its index (no PLAN-X) with the new mtime.
4. Every later open trusts the cache: the mtime matches, and PLAN-X is "not
found" until the cache file is deleted.

This is what happened with PLAN-3623CQ on 2026-10-12, and again during the T7
work. The design review found two more ways the cache goes stale without a
directory mtime change: a schema change (a new type for a directory that already
holds files, or a removed or renamed type), and watcher updates that change the
index without touching a directory (frontmatter id or type, a read error treated
as a removal). `cmd/rela-server` already works around the schema case by
deleting the cache file after a config reload (`DropStoreIndex`).

**Measurement (design review, warm APFS, tickets project with 8,640 entity files
and 10,413 relation files):** a full scan takes about 19 ms (7 ms entities, 12
ms relations; it lists directories and reads no file bodies). Reading and
parsing the 1.6 MB cache file takes about 22 ms, and every `Close` writes and
fsyncs 1.6 MB. The cache is slower than the work it saves.

**Scope:**
- In: remove the persisted entity and relation index. Every open scans.
Remove `persistedIndex`, `loadPersistedIndex`, `savePersistedIndex`, `IndexKey`,
`Config.IgnoreIndexCache` and its use in internal/app/factory.go,
`FSFactory.DropStoreIndex`, `appbuild.DropStoreIndex` (all four build-tag files)
and its call in cmd/rela-server/serve.go `retire()`.
- In: keep `Config.CacheKey`; it still locates `pending-deletes.json`.
- Out: deleting an existing `fsstore-index.json` left by older versions. It is
ignored from now on and lives in the gitignored `.rela/`; an older rela binary
run on the same project keeps using it, so removing it would not stop that
binary's bug anyway.
- Out: `cleanupTempFiles` deleting a `*.tmp` another process is about to
rename. Filed as a follow-up ticket.
- Out: the attachments index. It has no persisted cache (design review).

**Acceptance Criteria:**
1. The reproduction no longer loses an entity. Test: open store A, create an
entity through store B on the same directory, close B, close A, open store C: C
finds the entity. Same for a relation. On develop this test fails.
2. A schema change is picked up on the next open. Test: files exist in a
directory whose type is not in the layout; open with a layout that adds the
type: the entities are found.
3. `Close` writes no index file and `New` reads none. Test: after open and
close, no `fsstore-index.json` exists under the cache key; a planted stale index
file with a missing entity has no effect on the next open.
4. Pending deletes still work: the existing pending-deletes tests pass
unchanged.
5. Open cost is pinned: a benchmark (`BenchmarkOpen` over a generated tree of
a few thousand files) documents the scan cost; no budget assertion, since wall
time is machine-dependent.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small store fix)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: removing code)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A: covered by the code check)

**Research Doc:** N/A

**Existing Solutions:**
- Git's racy-git handling (Documentation/technical/racy-git.txt) shows what a
correct mtime-validated cache needs: per-entry validation against the index
file's own mtime, plus a schema key. That is far more machinery than a 19 ms
scan justifies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `syncIndex` calls `scanEntityDirs` and `scanRelationDir` directly.
2. Delete the persisted-index code and its tests; adapt tests that asserted
cache behavior to assert scan behavior, or delete them where they only tested
the cache.
3. `Close` no longer saves an index. It still stops the watcher, unwires
echoes and closes subscribers.
4. Remove `IgnoreIndexCache` from `fsstore.Config` and `app.FSFactory`
(read-only mode needed it only to avoid writing the cache).
5. Remove `DropStoreIndex` from internal/app and internal/appbuild and its call
in `retire()` in cmd/rela-server/serve.go; a config reload now needs no cache
step. Update comments that mention the index file.
6. Add the regression tests (AC1 to AC3) and the benchmark (AC5).

**Alternatives rejected:**
- Per-directory mtimes captured before the scan, with a racy window (the first
plan): fixes the concurrent-write race but keeps the schema-change and
watcher-update staleness, keeps a 1.6 MB write on every close, and is slower
than scanning.
- Add a schema hash to the cache key: fixes the schema case only.
- A cache keyed on a cheaper format (only the directory listing): still has to
list the directories to validate, which is the whole cost of the scan.

**Files to modify:**
- internal/store/fsstore/index.go, fsstore.go, watcher.go and their tests
- internal/app/factory.go (+ tests)
- internal/appbuild/appbuild_fs.go, appbuild_memory.go, appbuild_postgres.go,
appbuild_sqlite.go
- cmd/rela-server/serve.go (+ tests that reference DropStoreIndex)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The change removes an input: the cache file, which a hand edit or a stale
copy could make lie about which entities exist. The index now comes only from
the directory listing.

**Security-Sensitive Operations:**
- None added. One fewer file write per process.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each acceptance criterion names its test. AC1 uses two real
fsstore instances on one temp directory with the real FS, as two CLI processes
would.

**Edge Cases:**
- Missing entities or relations directory on first open: the scan already
treats not-exist as empty.
- Two types sharing a plural directory: unchanged scan behavior.
- Marked (pending-delete) families: hidden again from `pending-deletes.json`
on open, as before.

**Negative Tests:**
- A stale or corrupt `fsstore-index.json` present: ignored (AC3).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Cold opens (empty OS cache, slow disk, network share) were not measured. A
cold scan lists the directories; a cold cache read reads a 1.6 MB file, so the
scan is not expected to be slower, but large projects on slow storage should be
checked. The benchmark makes this measurable.
- A third-party embedder that sets `IgnoreIndexCache` or calls
`DropStoreIndex` stops compiling. These are internal packages, so only in-repo
callers exist.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore with no user-facing change)

**Documentation Impact:** N/A for user docs. Check docs-project and docs/ for
mentions of `fsstore-index.json` and remove them.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the RRs linked to TKT-33078A. The review's main
finding (the cache is slower than the scan) changed the approach from fixing the
cache to removing it, which resolves the race, schema and watcher findings.
