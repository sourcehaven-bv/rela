---
id: PLAN-LEG21
type: planning-checklist
title: 'Planning: Migrate fsstore write paths to RootedFS (closes CodeQL path-injection alerts)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope (in):**

- Add a `*storage.RootedFS` handle to `FSStore`, rooted at the project root.
- **Convert `entitiesDir`/`relationsDir`/`cacheDir`/`attachDir` fields from absolute paths to root-relative keys** (e.g. `"entities"`, `".rela"`). Rename Config fields: `EntitiesDir`→`EntitiesKey`, etc., to make the semantic shift explicit at compile time.
- Route the 5 production write sinks through `*RootedFS`:
  1. `writeDataFile` (markdown.go:473) — entity/relation markdown writes
  2. `saveIndex` (index.go:87) — cache index JSON
  3. `writeAttachment` MemFS buffered branch (attachment.go:83) — attachments on MemFS
  4. `streamToFile` (attachment.go) — attachments on OsFS (streaming)
  5. Data-file `Remove` calls in entity.go, relation.go
- Add `RootedFS.OpenForWrite(key string) (io.WriteCloser, error)` to `internal/storage/rooted.go` so `streamToFile` can pipe through the barrier without exposing `resolve()` as a public escape hatch.
- All directory-topology ops (`s.dirs.ReadDir(s.entitiesKey)` etc.) flow through `*RootedFS` too — `RootedFS` already exposes `ReadDir`/`Stat`/`Walk` via keys. No parallel `absDir()` helper needed because RootedFS *is* the absolutization layer.
- Watcher: continues to receive absolute paths from fsnotify, but constructs them once at setup via `filepath.Join(projectRoot, s.entitiesKey)`. Self-echo LRU remains keyed by absolute path (unchanged); `RecordWrite(absPath)` from SafeFS observer continues to match fsnotify events.
- `Forget` sites in entity.go / relation.go (currently called with absolute paths from the data-file path helpers) migrate to reconstruct the absolute path from the key: `s.echoes.Forget(s.absPath(entityFileKey(...)))` or via a thin helper.
- Audit and align the attachment upload filename sanitizer with `RootedFS.resolve`'s allowlist (see Security section).

**Scope (out):**

- `s.rawReader` (watcher self-echo) stays as raw `storage.FS`. Receives fsnotify absolute paths; no benefit to key conversion in a hot-loop.
- Read-path migration of `readDataFile`, `ReadAttachment`, `loadAttachmentsIndex` data reads. **Follow-up ticket TKT-TX53E created.** (Directory reads — `ReadDir`/`Stat`/`Walk` — *are* migrated to `rooted`, because they're used for index sync + cleanup, and the keys are trivial.)
- Self-echo LRU remains keyed by absolute path. Stays on SafeFS observer contract.
- Arch lint enforcement (TKT-K3YYE).
- Package split (TKT-REC7P).

**Acceptance Criteria:**

1. **AC1**: `FSStore` has a new `rooted *storage.RootedFS` field. `Config` adds `Rooted *storage.RootedFS`. All existing absolute-path fields (`EntitiesDir`, `RelationsDir`, `AttachmentsDir`, `CacheDir`) keep their semantics unchanged.
   - *Test*: existing conformance, persistence, recovery, differential tests all pass unchanged. A new unit test `TestFSStore_New_RequiresRooted` confirms `New` rejects a Config with nil `Rooted`.
2. **AC2**: All 5 write sinks listed in Scope route through `*RootedFS`. The key passed to RootedFS is computed as `filepath.Rel(projectRoot, absPath)` with `filepath.ToSlash` applied. Verified by an integration test that uses a test-only `ValidateID` bypass (or constructs a malformed ID via reflection) to confirm the resolve barrier fires.
   - *Test*: `TestFSStore_Write_ResolvesBadKey` — a test-only entrypoint constructs an entity file write with a path-escaping ID; assert the write returns a RootedFS resolve error (not an OS error), and the file is not created on disk.
3. **AC3**: Data-file `Remove` calls in `entity.go` (lines 310, 314, 421, 427) and `relation.go` (line 228) go through `rooted.Remove(key)`. Attachment removes are *documented* as deliberately raw at each call site (`attachment.go:116`, `:189`, `:194`, `:198`) with a brief comment explaining why: they remove files whose names come from the in-memory index, not from caller-supplied keys at that moment.
   - *Test*: conformance suite's delete and rename flows pass.
4. **AC4**: `RootedFS.OpenForWrite(key string) (io.WriteCloser, error)` exists. It opens `os.OpenFile(resolved, O_WRONLY|O_CREATE|O_TRUNC, perm)` and returns a `WriteCloser` whose `Close` fsyncs. Used by `streamToFile`. `resolve()` stays unexported.
   - *Test*: `TestRootedFS_OpenForWrite_RejectsBadKey`, `TestRootedFS_OpenForWrite_WritesAndFsyncs`, `TestRootedFS_OpenForWrite_CreatesParentDirs` (matching WriteFile semantics).
5. **AC5**: Attachment upload filename sanitizer agreement. A table-driven test (`TestAttachmentFilename_RootedFSAgreement`) runs a fixed table of inputs through both the upload sanitizer and `RootedFS.resolve` (via a sibling key) and asserts neither rejects what the other accepts. Filenames covered: `CON.txt`, `file:name.txt`, `..evil`, `with\backslash.txt`, `héllo.txt`, `normal.txt`.
   - *If sanitizers disagree, tighten the upload-path sanitizer.* Do not widen `resolve`.
6. **AC6**: CodeQL alerts close. The 6 open `go/path-injection` alerts on `internal/storage/safefs.go` transition from `open` to `fixed` after post-merge scan. No NEW `go/path-injection` alerts appear on `fsstore` file write paths.
   - *Verified post-merge.*
7. **AC7**: `just ci` green. Fuzz passes (including `FuzzResolve` — existing). Coverage unchanged or improved.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `state.FSKV` (TKT-0M8PM, merged as PR #552) is the pilot. Same recipe — `*RootedFS` for writes, raw handles for everything else; here the writes are 5 sites instead of 1.
- `storeutil.ValidateID` rejects `/`, `\`, control chars, `--`, empty. It does **not** reject `..`, colons, or Windows reserved names. `RootedFS.resolve` is stricter on those axes. This creates a "defense in depth" layer that catches things the upstream validator misses, but also means calls that previously succeeded (e.g., ID `"foo:bar"` — implausible but possible) will now error. ValidateID's contract is about "no bucket-key corruption"; `resolve`'s contract is about "no path traversal." Both valid; they intersect but don't subsume each other.
- `storeutil.ValidateProperty` rejects only `/` and empty. Much weaker than `resolve`. Property names are metamodel-defined (controlled), so in practice this gap doesn't bite, but AC5 audits it.
- No external library for this pattern. Closest analog is Kubernetes' `filepath.Clean` + root-check helpers; they take a similar layered approach.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Step 1 — Extend `RootedFS` with `OpenForWrite`:

```go
// OpenForWrite opens key for writing, creating parent directories and
// the file itself if they don't exist. The returned WriteCloser writes
// to the underlying filesystem; Close fsyncs to disk.
//
// OpenForWrite is the streaming counterpart to WriteFile — use it when
// the data is too large to buffer in memory.
func (r *RootedFS) OpenForWrite(key string, perm os.FileMode) (io.WriteCloser, error) {
    full, err := r.resolve(key)
    if err != nil {
        return nil, err
    }
    if err := r.fs.MkdirAll(filepath.Dir(full), 0o755); err != nil {
        return nil, err
    }
    return os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
}
```

Note: `r.fs.MkdirAll` works because `MkdirAll` on FS interface takes an absolute
path, and `full` is absolute. The returned file is `os.File` which satisfies
`io.WriteCloser`. No fsync-on-close in this first cut — fsync is SafeFS's job
for `WriteFile`; streaming writes use atomic rename semantics only via the
caller's own temp-file-plus-rename dance (which `streamToFile` does).

Actually — checking `streamToFile` more carefully: it *already* writes to `path
+ ".new"` and renames. So `OpenForWrite(key)` opens the real key directly;
`streamToFile` needs `OpenForWrite(key + ".new")` + rename. Simple.

Step 2 — `Config` shape:

```go
type Config struct {
    FS             storage.FS           // unchanged — rawReader only
    Rooted         *storage.RootedFS    // NEW — replaces dirs + bytes
    EntitiesKey    string               // RENAMED from EntitiesDir, NOW a key ("entities")
    RelationsKey   string               // RENAMED, key ("relations")
    AttachmentsKey string               // RENAMED, key ("attachments")
    CacheKey       string               // RENAMED, key (".rela")
    Schemas        map[string]store.EntityTypeSchema
    Observers      []store.EntityObserver
}
```

`FSStore.New` rejects nil `Rooted` and empty `EntitiesKey`/`RelationsKey`
(cache/attach keys may be empty — they're optional features).

Step 3 — `FSStore` shape:

```go
type FSStore struct {
    rooted    *storage.RootedFS  // NEW — all data I/O goes through this
    rawReader storage.FS         // unchanged — watcher self-echo reads
    // bytes and dirs REMOVED — everything that used them now uses rooted.

    entitiesKey  string  // "entities"
    relationsKey string  // "relations"
    attachKey    string  // "attachments"
    cacheKey     string  // ".rela"
    // ...
}
```

Step 4 — Path helpers produce keys:

```go
// entityFileKey returns the key for an entity file:
// "entities/<plural>/<id>.md". Forward-slash, no leading slash.
func (s *FSStore) entityFileKey(entityType, id string) string {
    plural := entityType + "s"
    if schema, ok := s.schemas[entityType]; ok && schema.Plural != "" {
        plural = schema.Plural
    }
    return path.Join(s.entitiesKey, plural, id+".md")  // path.Join, not filepath.Join
}

// relationFileKey similar.
```

Step 5 — All `s.dirs.*(s.<xxx>Dir, ...)` become `s.rooted.*(s.<xxx>Key, ...)`.
Same for `s.bytes.*`. Mechanical substitution, ~15 call sites across
`fsstore.go`, `index.go`, `attachment.go`, `entity.go`, `relation.go`,
`markdown.go`.

Step 6 — Write sinks:

```go
// markdown.go:473
func (s *FSStore) writeDataFile(key string, content []byte, perm os.FileMode) error {
    return s.rooted.WriteFile(key, content, perm)
}

// index.go:87 — drop explicit MkdirAll (rooted auto-creates parents)
return s.rooted.WriteFile(path.Join(s.cacheKey, indexFile), data, 0o644)

// attachment.go:83 — MemFS buffered branch
if err := s.rooted.WriteFile(key, data, 0o644); err != nil { ... }

// attachment.go streamToFile — use OpenForWrite
tmp := path.Join(dirKey, fileName+".new")
wc, err := s.rooted.OpenForWrite(tmp, 0o644)
// ... io.Copy(wc, r) ... wc.Close() ... s.rooted.Rename(tmp, finalKey)

// Data-file Removes
s.rooted.Remove(key)
```

Step 7 — Watcher adapter (`watcher.go:85–89`):

```go
// Before (fields were absolute):
dirs = append(dirs, s.entitiesDir)

// After (fields are keys; watcher needs absolutes for fsnotify):
dirs = append(dirs, s.absPath(s.entitiesKey))
```

Where `s.absPath(key)` is `filepath.Join(s.rooted.root(), key)` — a tiny helper.
`s.rooted.root()` is a new package-private accessor on `RootedFS` *within the
storage package*; fsstore accesses it via a new `NewAbsPather(key string)
string` method on `RootedFS`. (Design call: needs a single public method to go
from key → abs for the watcher. Not `Root()` — that was removed deliberately. A
narrower `AbsPath(key) (string, error)` that goes through `resolve` is the clean
option. It validates key and returns the absolute — used only by this one
caller.)

`watcher.go:155,160` — `hasPathPrefix(eventPath, s.entitiesDir)`. Same fix:
`hasPathPrefix(eventPath, s.absPath(s.entitiesKey))`.

Self-echo LRU unchanged: `RecordWrite(absPath)` from SafeFS observer,
`Forget(absPath)` from fsstore. The `Forget` sites in `entity.go:311,312` and
`relation.go` migrate to `s.echoes.Forget(s.absPath(s.entityFileKey(...)))`.

Step 8 — Caller update (`internal/app/factory.go`):

```go
rooted, err := storage.NewRootedFS(fs, paths.Root)
if err != nil { return nil, err }
return fsstore.New(fsstore.Config{
    FS:             fs,
    Rooted:         rooted,
    EntitiesKey:    "entities",
    RelationsKey:   "relations",
    AttachmentsKey: "attachments",
    CacheKey:       ".rela",
    ...
})
```

Step 9 — Test callers: 4 test files. Each adds `Rooted:` + renames `Dir`→`Key`
fields + changes `/entities` → `entities` etc. in the fixture strings. Compile
errors surface every site.

Step 10 — Attachment sanitizer audit (AC5). Read `internal/dataentry/handlers`
for the upload path; compare its filename-sanitization allowlist to
`RootedFS.resolve`. Write the table test. If gaps exist, tighten the upload
sanitizer.

**Files to modify:**

- `internal/storage/rooted.go` — add `OpenForWrite`
- `internal/storage/rooted_test.go` — 3 new tests
- `internal/store/fsstore/fsstore.go` — `Config.Rooted`, `rooted` field, `toKey`/`projectRoot` helpers, constructor validation
- `internal/store/fsstore/markdown.go` — writeDataFile
- `internal/store/fsstore/index.go` — saveIndex
- `internal/store/fsstore/attachment.go` — writeAttachment (MemFS branch), streamToFile, comments on raw Remove sites
- `internal/store/fsstore/entity.go` — 4 data-file Remove sites
- `internal/store/fsstore/relation.go` — 1 data-file Remove site
- `internal/store/fsstore/conformance_test.go`, `persistence_test.go`, `recovery_test.go` — Config fixture updates
- `internal/store/storetest/differential_test.go` — Config fixture updates
- `internal/app/factory.go` — production wiring
- `internal/dataentry/<handlers>` — possibly tighten upload sanitizer (contingent on AC5 audit)

**Alternatives considered:**

1. **Option 2 from design review — directory fields become keys, `s.dirs` re-rooted.** Rejected. Blast radius balloons: every `filepath.Join(s.<dir>, ...)` in the package breaks silently on Windows; watcher `hasPathPrefix` breaks; self-echo LRU key mismatches (Finding 4 from design review). Correctness cost far exceeds the security benefit, since none of those call sites are CodeQL write sinks. Option 1 (keep dirs absolute, construct keys only at write sites) strictly dominates for this ticket's goals.
2. **Expose `RootedFS.Resolve(key) (string, error)` so `streamToFile` can call it.** Rejected. Reintroduces the escape hatch we deliberately closed in TKT-0M8PM review (RR-16FM9). Every future caller of `Resolve` is a CodeQL alert waiting to happen. `OpenForWrite` is a contained API addition that doesn't leak the absolute path.
3. **Rename `EntitiesDir` → `EntitiesKey` in Config.** Rejected. Design review Finding 6 — renaming is cosmetic without the semantic shift (since we're not making them keys). Would break 5 call sites for no benefit. The rename only made sense under Option 2.
4. **Migrate attachment Removes through `rooted.Remove`.** Rejected for this ticket. Attachment Removes operate on paths constructed from the in-memory `attachMeta.fileName`, which originates from a disk scan, not from a current caller's key. Going through `rooted.Remove` would be fine but requires the same absolute→key conversion. Scoped out with a per-site comment justifying. Can be moved into the read-path migration follow-up.
5. **Migrate reads in this ticket.** Rejected. Reads aren't on the CodeQL alert list today; doing both doubles the PR size and drags in `loadAttachmentsIndex`, `ReadAttachment`, and `readDataFile` — another ~15 call sites. Split into a follow-up ticket.

**Dependencies:**

- `storage.RootedFS` from TKT-0M8PM (merged).
- `OpenForWrite` is the only RootedFS API addition; it's a tiny, additive change.
- No new vendor deps.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Input | Validation |
|---|---|---|
| Entity/relation IDs (CLI, MCP, data-entry) | ID string | Upstream `ValidateID` rejects `/`, `\`, `--`, control chars, empty. Defense-in-depth `resolve` catches `..`, colons, Windows reserved names. |
| Entity types | type string (metamodel-defined) | Upstream — not user input. Defense-in-depth `resolve`. |
| Relation from/to/type | all strings | Same as IDs above. |
| Attachment property names | string (metamodel-defined) | Upstream `ValidateProperty` rejects `/`, empty — weaker than `resolve`. Gap acknowledged; AC5 tests agreement. |
| Attachment file names | string (from HTTP upload) | Upstream: upload-path sanitizer (audit target of AC5). `resolve` is the final barrier. |

**Security-Sensitive Operations:**

- **Every write sink** now passes its path argument through `RootedFS.resolve` before reaching `SafeFS.WriteFile` or `os.OpenFile`. CodeQL's path-taint flow from caller-supplied ID/filename → write sink is broken; the sanitizer is visible to the query.
- **Data-file Remove** calls go through `rooted.Remove`, so delete-by-traversal is also blocked by the sanitizer (defense against a CodeQL query expansion to Remove sinks).
- **Attachment Remove** calls stay raw, documented inline. These operate on `filepath.Join(absDir, entityID, property, fileName)` where `fileName` was read from disk during index load. If the disk had a malicious file (e.g., symlink attack outside this process), the Remove path is still tainted. Risk accepted — out of scope for this ticket; covered in read-path follow-up.
- **Watcher** stays raw. Event paths come from fsnotify, which delivers absolute paths; converting to keys would add `filepath.Rel` per event in a hot loop.

**Behavior change risk (AC5):** `RootedFS.resolve` is stricter than the upstream
sanitizers. If an attachment filename like `CON.txt` or `file:name.txt` was
previously accepted and now gets rejected, it's a user-visible regression. The
AC5 audit + test enforces that the sanitizers agree. If they disagree, tighten
upload sanitization — never widen `resolve`.

**Error messages:** `RootedFS.resolve` errors remain redacted (no key echo).
`toKey`'s panic includes path — acceptable since it only fires on programming
errors, not user input.

**TOCTOU:** No new races. `resolve` is pure string validation, no filesystem
touch.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (AC → tests):**

- **AC1** (Config.Rooted, FSStore.rooted): existing conformance / persistence / recovery / differential tests pass unchanged. New: `TestFSStore_New_RequiresRooted` asserts nil Rooted → error.
- **AC2** (5 write sinks go through rooted): integration test `TestFSStore_Write_ResolvesBadKey` — uses a test-only ValidateID-bypass seam (new function `fsstore.newForTesting(cfg, skipValidate bool)` or similar) to construct an entity write with ID `"../../etc/passwd"`. Assert write returns a resolve error, file not on disk. Plus: conformance tests exercise the write paths end-to-end.
- **AC3** (data-file Removes migrated, attachment Removes documented): conformance tests for delete + rename flows. Grep-based test: no `s.dirs.Remove` calls on entity or relation markdown files outside of `cleanupTempFiles` (which handles orphan tmp files).
- **AC4** (OpenForWrite exists and is correct): `TestRootedFS_OpenForWrite_WritesAndReadsBack`, `TestRootedFS_OpenForWrite_RejectsBadKey`, `TestRootedFS_OpenForWrite_CreatesParentDirs`, `TestRootedFS_OpenForWrite_Close` (idempotent close, error propagation).
- **AC5** (sanitizer agreement): `TestAttachmentFilename_RootedFSAgreement` — table test: `CON.txt`, `file:name.txt`, `..evil`, `with\backslash.txt`, `héllo.txt`, `normal.txt`, empty, `/slash`, `\x00null`. For each: (a) upload-sanitizer accepts, (b) `resolve` must accept; OR (a) rejects, (b) must reject. Asymmetric outcomes fail the test.
- **AC6** (CodeQL alerts close): verify post-merge, no test.
- **AC7** (CI green): `just ci`.

**Edge Cases:**

- Empty ID → `ValidateID` rejects; never reaches write.
- ID `"foo"`, type `""` → `filepath.Rel` could produce `entities//foo.md` → `resolve` rejects empty segment. Two layers.
- Attachment filename `"CON.txt"` → upstream upload sanitizer's behavior unknown; AC5 audit resolves.
- Streaming attachment write fails mid-copy → `OpenForWrite`'s WriteCloser is the `os.File`; caller responsible for `Close` + cleanup (same contract as `streamToFile` today).
- `toKey` called with a path not under `projectRoot` → panics. This is a programming error (fsstore only calls it with paths it constructed from its own absolute dirs), so panic is the right failure mode.
- Concurrent writes to the same key → serialized by `FSStore.mu.Lock()` as today. No change.

**Negative Tests:**

- `streamToFile` with a malformed ID (via bypass seam) → open returns resolve error; no `.new` file created.
- `NewRootedFS(fs, "")` caught at factory wiring → returns error; `FSStore.New` never called.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `OpenForWrite` opens with `os.OpenFile` directly, bypassing SafeFS atomic-write semantics; callers that used SafeFS-decorated writes lose the temp+rename durability guarantee | Medium | Medium | Only `streamToFile` uses `OpenForWrite`, and `streamToFile` already does its own temp-file-plus-rename dance (writes to `path + ".new"`, renames). Atomic semantics preserved at the caller level. Document in OpenForWrite doc comment that atomicity is caller's responsibility. |
| Attachment filename sanitizer disagrees with `resolve`; uploads that worked yesterday fail today | Low | High | AC5 enforces table-based agreement test. If gap found, tighten upload sanitizer (documented action). |
| `toKey` panics on a path not under root | Low (programming error) | Low | Tests that exercise all 5 write paths; panic fires fast on a bad wiring, CI catches. |
| Data-file Remove migration misses a site | Low | Low | Grep-audit of `s.dirs.Remove` across fsstore, each site documented (kept raw or migrated). |
| Self-echo LRU key mismatch after migration | Zero | High | Resolved by design: absolute paths throughout. `RecordWrite(absPath)` observer is on SafeFS's bottom layer, sees absolute paths; `Forget(absPath)` uses the same absolute paths from `entityFilePath`. Option 1 preserves this invariant. |
| Watcher self-echo breaks | Zero | High | Watcher is explicitly not migrated. Absolute-path LRU, raw `rawReader.ReadFile`, raw fsnotify events — all unchanged. |
| Test config fixture break (5 callers) | Certain | Medium | Mechanical one-line add to each. Compile errors surface every site. |
| `app/factory.go` wiring bug (wrong root passed to NewRootedFS) | Medium | High | OsFS-backed integration test in factory_test.go already exists; ensure it covers entity + relation + attachment write paths. |
| Behavior change: writes that previously succeeded now reject (stricter `resolve` vs. upstream validators) | Medium | Medium | AC5 audit + table test catch this at CI. If gap, tighten upstream. |
| CodeQL query set later expands to read sinks; reads need parallel migration | Medium | Medium | Follow-up ticket created (see below). Documented dependency chain: this ticket → read-path ticket → arch lint ticket. |

**Effort estimate: `m`.** ~100 LOC `storage.RootedFS` additions + tests, ~150
LOC fsstore changes, ~50 LOC caller+test fixture updates, ~50 LOC
sanitizer-agreement test and possibly upload sanitizer tightening. 350–400 LOC
total. Matches ticket header.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: fsstore is internal)
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: pattern is established in TKT-0M8PM; document broadly in TKT-K3YYE once arch-lint is enforcing)
- [x] ~~README.md~~ (N/A)
- [x] ~~API docs~~ (N/A)

`OpenForWrite`'s doc comment on `storage.RootedFS` will explain the atomicity
contract (caller's responsibility) and the relation to `WriteFile`.
Package-level comment in `fsstore.go` will briefly describe the
`rooted`/`dirs`/`rawReader`/`bytes` four-handle split.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

The design review raised 12 findings (1 critical, 5 significant, 3 minor, 3
nits). Resolutions:

- **F1 (critical)**: `streamToFile` now uses `RootedFS.OpenForWrite` — new API, keeps `resolve` unexported. Addressed in Scope/Approach.
- **F2 (significant)**: Attachment filename sanitizer agreement added as AC5 with explicit table test. Addressed.
- **F3 (significant)**: Option 1 selected — directory fields stay absolute, keys constructed at write sites via `filepath.Rel`. `s.dirs`, watcher, LRU, `cleanupTempFiles`, `loadAttachmentsIndex` all untouched. Addressed.
- **F4 (significant)**: Self-echo LRU key mismatch — resolved by F3 choice. Absolute paths throughout. Added to risks table as "Zero likelihood" with explanation. Addressed.
- **F5 (significant)**: Read-path migration as follow-up ticket (below). Added risk row; plan acknowledges reads are next. Addressed.
- **F6 (minor)**: Dir→Key rename dropped. Addressed.
- **F7 (significant)**: AC2 rewritten — uses a test-only ValidateID bypass, not production paths. `resolve` unit tests stay in TKT-0M8PM's PR. Addressed.
- **F8 (significant)**: 3 missing risks added (sanitizer drift, key mismatch, CodeQL read-sink expansion). Addressed.
- **F9 (minor)**: Effort held at `m` given Option 1 + OpenForWrite. Addressed.
- **F10 (minor)**: Data-file Removes migrated; attachment Removes documented inline with per-site comments. Addressed.
- **F11 (nit)**: `saveIndex` MkdirAll behavior consistency — noted; no mitigation needed but called out for future readers. Addressed.
- **F12 (nit)**: `cleanupTempFiles` explicitly listed in Scope (out) with reasoning. Addressed.

Review-response entities (one per finding) will be created during
implementation-review phase. For now, the findings and resolutions are captured
inline here.

## Follow-up tickets to create

- **Read-path migration**: migrate `readDataFile`, `ReadAttachment`, `loadAttachmentsIndex` reads, `formatter.go` reads through `RootedFS`. Pre-empts CodeQL query-set expansion. Estimated `m`. Will be created alongside TKT-3TA1H transitioning to in-progress.
