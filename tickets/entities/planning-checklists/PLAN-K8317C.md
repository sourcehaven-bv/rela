---
id: PLAN-K8317C
type: planning-checklist
title: 'Planning: fsstore temp-file cleanup can delete another process''s in-flight write'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `storage.SafeFS.WriteFile` (internal/storage/safefs.go) writes to
the fixed name `path + ".tmp"`, then fsyncs and renames it. `fsstore.New` calls
`cleanupTempFiles` (internal/store/fsstore/fsstore.go), which deletes every
`*.tmp` and `*.new` under the entity and relation directories. Two failures
follow when several rela processes share a project (several agents running the
CLI, or the CLI next to a server):

1. A process that opens a store deletes the temp file of another process that
is between its write and its rename; that rename fails, so the write fails.
2. Two processes writing the same file at once open the same temp file with
`O_TRUNC`; one can rename the other's half-written bytes into place.

**Scope:**
- In: unique temp names in `SafeFS.WriteFile`; a grace period in
`cleanupTempFiles` so it removes only stale temp files.
- In: attachment uploads (internal/store/fsstore/attachment.go). Their temp
name is a prefix plus a hash of the file name, the same for every writer, so two
processes uploading the same name share one temp file.
- In: `rela gc --temp-files` (`analysis.FindOrphanedTempFiles`) and the
fsimport skip, which must recognize the new names.
- Out: any cross-process write lock. Two processes writing the same entity
still race on the content (last rename wins), as today; this ticket only makes
each write whole and stops one process from failing another's write.

**Acceptance Criteria:**
1. Each `SafeFS.WriteFile` call uses its own temp file in the target's
directory, ending in `.tmp`. Test: two concurrent writes of the same path with
different content, many iterations: every final file equals one of the two
inputs exactly (never a mix), and no write returns an error.
2. The written file has the same permissions as today: the requested mode
with the umask applied. Test: write with 0o644 and check the mode equals `0o644
&^ umask`.
3. `cleanupTempFiles` leaves a fresh temp file alone and removes a stale one.
Test: one temp file with a recent mtime and one made stale with `os.Chtimes`;
open a store; the fresh file remains, the stale one is gone. Cases for the new
form, the old `.tmp` form and `.new`.
4. `rela gc --temp-files` finds stale `.tmp` and `.new` leftovers and skips
fresh ones. Test on `analysis.FindOrphanedTempFiles`.
5. fsimport does not copy a temp file under `.rela/` into the state KV. Test
on `classifyRela` (or the import of a tree with a leftover).
6. Two concurrent attachment uploads of the same file name to the same entity
both succeed. Test with two goroutines.
7. A failed write leaves no temp file behind (existing behavior, kept). Test:
make the rename fail (target is a directory); no `*.tmp` remains.
8. The scan and the watcher ignore the new temp names. Test: a temp file named
like the new pattern in an entity directory does not show up as an entity (the
filters already require a `.md` suffix; the test pins it).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small fix)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: `os.CreateTemp` is the standard tool)
- [x] ~~Reviewed relevant rela concepts for prior art~~ (N/A)

**Research Doc:** N/A

**Existing Solutions:**
- `os.CreateTemp(dir, pattern)` creates a file with a unique random name and
`O_EXCL`, so two writers never share one. Libraries such as
`github.com/google/renameio` and `natefinch/atomic` do the same (CreateTemp in
the target directory, fsync, rename); adding a dependency for this is not worth
it.
- The scan (index.go) and the watcher (watcher.go) only consider names ending
in `.md`, so a name like `X.md.123456.tmp` is ignored.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. **One shared temp-file rule in `internal/storage`:** a function that builds
a temp name (`<base>.<random>.tmp`, random from crypto/rand, hex), a predicate
that recognizes temp names (the new form, the old fixed `<base>.tmp`, and the
legacy `.new`), and a stale-after rule (mtime older than 10 minutes). Every
sweep and skip uses these, so they cannot drift.
2. `SafeFS.WriteFile`: open the temp file with
`os.OpenFile(tmp, O_WRONLY|O_CREATE|O_EXCL, perm)`, retrying with a new random
name on `ErrExist` (a few attempts). This keeps exclusive creation, honours the
umask exactly as today, and needs no chmod (design review: `os.CreateTemp` plus
`Chmod` would ignore the umask). Then write, fsync, close, rename, fsync the
directory; on any error remove that temp file.
3. `cleanupTempFiles`: remove only names the shared predicate recognizes and
whose mtime is stale. A write takes milliseconds even with fsync, so a stale
file is a crash leftover.
4. `analysis.FindOrphanedTempFiles` (used by `rela gc --temp-files`) uses the
same predicate and stale rule, so it also finds `.tmp` leftovers and never
reports a live write.
5. fsimport's `classifyRela` skips temp names, so a leftover under `.rela/`
(theme, migration state) is never copied into the target state KV.
6. Attachment uploads: give the attachment temp name a random part too, so two
processes attaching the same file name to the same entity no longer share one
temp file; keep its prefix so the attachment index still skips it.
7. Update the `cleanupTempFiles` and `SafeFS.WriteFile` comments.

**Alternatives rejected:**
- A cross-process lock file around writes: fixes more (content races), but is
a larger change with stale-lock handling; not needed for this bug.
- Removing the cleanup: crash leftovers would pile up; the grace period keeps
the cleanup and makes it safe.
- Matching only temp names of the old fixed form: old binaries still write
them, so the cleanup must keep handling both forms; the suffix check stays.

**Files to modify:**
- internal/storage/safefs.go and a new internal/storage/tempfile.go (+ tests)
- internal/store/fsstore/fsstore.go (`cleanupTempFiles`) and attachment.go
(+ tests)
- internal/analysis (FindOrphanedTempFiles) (+ test)
- internal/fsimport (classifyRela skip) (+ test)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The temp name comes from `os.CreateTemp` in the target's own directory; the
target path is already validated by the callers (rooted FS). No new path input.

**Security-Sensitive Operations:**
- File permissions: the temp file is created with the requested mode and the
process umask, exactly as today, so no file becomes more open.
- `O_EXCL` also prevents following a planted symlink at the temp name, which
the fixed `.tmp` name with `O_CREATE|O_TRUNC` did not.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each acceptance criterion names its test. AC1 runs real
concurrent writes on the OS filesystem with `-race`.

**Edge Cases:**
- Long file names: the suffix adds about 21 bytes (a dot, 16 hex characters,
`.tmp`). Relation file names (`FROM@face--TYPE--TO@face.md`), comment keys and
channel names are the longest; check the longest the code allows against the
255-byte name limit, and if a name could exceed it, truncate the base part of
the temp name (the temp name need not contain the full base).
- Old-form `X.md.tmp` leftovers: still cleaned up once stale.
- A temp file whose mtime is in the future (clock skew): treated as fresh,
so it is kept; it is removed on a later open once old enough.

**Negative Tests:**
- Rename failure leaves no temp file (AC4).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Random names remove the self-healing of the fixed name: a crash leftover is
no longer overwritten by the next write of the same file. In entities/,
relations/ and attachments the startup sweep removes stale leftovers, and `rela
gc --temp-files` reports them. Elsewhere (`.rela/` state, comments, channels,
config) nothing sweeps them; fsimport skips them. A leftover only appears after
a crash between create and rename, which is rare, and it is ignored by every
reader. `filechannels` `pruneEmpty` cannot remove a directory that still holds a
leftover; that costs an empty directory, not data.
- Mixed versions: an older binary still writes the fixed name and its cleanup
still deletes everything; the fix protects only processes running the new code.
Acceptable; noted.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, no user-facing change)

**Documentation Impact:** N/A; code comments only.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the RRs linked to TKT-H63QDR. The two
significant findings changed the plan: one shared temp-file rule used by every
sweep and skip (fsstore, `rela gc`, fsimport), and `O_EXCL` with the requested
mode instead of `CreateTemp` plus `Chmod`, so the umask still applies.
