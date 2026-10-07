---
id: PLAN-GRM81I
type: planning-checklist
title: 'Planning: fs-to-sqlite migration command'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Integration sync (FEAT-XYQMUB) needs store-level history, which only the
database backends have. An fs project today can only move to sqlite by exporting
and re-creating its data. This ticket adds a one-way copy of an fs project into
a NEW sqlite project directory that the user chooses. The fs project is never
modified and stays the backup.

**Scope:**

In scope:
- `rela db import-fs <source> <target>` in the sqlite build (`rela-sqlite`).
Other builds answer that the command needs the sqlite build. `<target>` must not
exist; its parent must.
- Copy into `<target>/.rela/rela.db`:
  - entities and relations, every face; the id is the one fsstore uses (the
file name, see `fsstore/markdown.go` "The FILENAME is the identity");
  - attachments, whose property values stay valid unchanged;
  - comments, for imported entities only;
  - applied data-migration state, read the way appbuild reads it
(`datamigration.NewLegacyBridge`);
  - runtime state keys and key prefixes (`.rela/<key>` to `state_kv`), from an
explicit classification of every `.rela/` entry (see Approach).
- Copy the project files to `<target>`: everything EXCEPT `entities/`,
`relations/`, `attachments/`, `.git/`, `migrations/applied.json` and `.rela/`.
From `.rela/`, copy only the config files `config.yaml`, `mail.yaml`, `ai.yaml`,
`secrets.yaml`, and `audit/`.
- Ensure the target's `.gitignore` covers `.rela/`.
- Normalize property values to the form sqlite stores (JSON); see Approach.
- Run the derived-index reconcile on the new database.
- Report: counts, every source file NOT copied with its reason, dropped
soft-deleted ids, orphan comment threads, normalized values, a validation report
(does not block), and manual follow-ups (re-run `rela secrets credential-name`
from the final location; set up git if wanted; plaintext warning when the source
used git-crypt).
- Fix the stale docs: `docs/sqlite-backend.md` and the comment in
`internal/appbuild/statekv_nodb.go` still say sqlite keeps runtime state in
`.rela/` files (TKT-STATSQL moved it into the database).

Out of scope:
- In-place conversion, sqlite to fs, postgres as a target.
- Importing git history. Versions start at the migration.
- Storing config in the database (`project_files`, FEAT-UP14BT packaging).
- Pending (undoable) deletes: not copied; their ids are listed.
- Desktop backend choice: separate ticket.
- Reviving `rela sync` (removed in cf0fe87e).

**Acceptance Criteria:**
1. Migrating a fixture project produces a target that `rela-sqlite` opens.
Every entity and relation has the same canonical hash (`canonical.HashEntity` /
`HashRelation`) on both sides, after the documented value normalization;
attachments are byte-identical; comments keep ids, authors and timestamps; the
applied-migration state and the copied state keys are equal. Verified against
the FINAL `rela.db`, reopened read-only after the rename.
2. The source tree is byte-identical before and after a run, successful or
not (no temp-file cleanup, no index cache write, no database opened).
3. A git-crypt-encrypted entity, relation or attachment refuses the whole run
and lists every one. The check also runs in the write loop and in verify, so
content that becomes locked mid-run still fails the run.
4. An existing target, a target inside the source, or a source inside the
target is refused before anything is written, also on a case-insensitive
filesystem.
5. Any failure leaves neither the target nor the staging directory behind.
6. `rela-sqlite migrate status` on the target shows nothing pending that was
applied in the source, including a project that still has the legacy
`migration/state.json` marker.
7. Every copied row is attributed to tool `fs-import`, and the run leaves one
audit record in the target's audit log.
8. Row-level problems (ids colliding case-insensitively, the same id in two
type folders, invalid ids, values that cannot be stored, oversized attachments,
relation file names that disagree with their frontmatter) are ALL collected and
reported in one run, and nothing is kept. Entities that only fail schema
validation are copied and listed.
9. Files fsstore does not read (undeclared type folders, unparsable file
names) are listed in the report, by path and reason.
10. `.rela/` in the target is mode 0700, `secrets.yaml` 0600, audit files
0600; the target `.gitignore` covers `.rela/`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach settled in discussion with the user; no competing designs left)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: rela-specific store formats; no external tool reads both)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- No store-to-store copy exists. `rela sync` (fs to remote server over HTTP)
was removed in cf0fe87e (TKT-7IZHP0, ruling D9: unused) and covered only
entities and relations.
- `rela dev seed` (`internal/cli/dev.go`, `internal/perfseed/load.go`) is the
pattern for a raw-store write: `store.WithAttribution`, batched `st.Tx`, one
audit record (also on failure).
- `internal/canonical` is the cross-backend content comparison;
`store.VersionOf` is not (it hashes Go types, `store/version.go:130`).
- sqlite tier wiring: `appbuild/appbuild_sqlite.go:openBackend`,
`appbuild/configloader_sqlite.go:backendServices`.
- Migration state: `datamigration.NewLegacyBridge` (`appbuild/migstate.go`).
- Comments: `filecomments` already walks its thread keys
(`threadKeysFor`); `comments.Store.Add` keeps ID, author and timestamp.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Paths.** Resolve source; resolve the target's parent with
`EvalSymlinks` and append the base name. Refuse when the target exists, or when
either path is an ancestor of the other, tested by walking ancestors with
`os.SameFile` (case-insensitive safe). Refuse a source without `schema.yaml`,
and a source with `.rela/rela.db` (plain `stat`, never opened).
2. **Open the source read-only.** fsstore over a read-only `storage.FS`
wrapper whose write methods fail, plus a new fsstore option that bypasses the
persisted index cache (separate from `CacheKey`, which also drives
pending-delete reads). Record the source freshness (tree hash of the data
folders) at start.
3. **Staging.** Create a sibling directory `<target-parent>/.<name>.import-<rand>`
with `os.Mkdir` (mode 0700 for `.rela/`). Everything is built there; the
database is opened at its final relative path `.rela/rela.db`.
4. **Copy project files** with `os.OpenRoot` on source and staging: regular
files and directories only, no symlinks or special files (listed), modes without
setuid/setgid/sticky. `.rela/` config files from the allowlist; `secrets.yaml`
written 0600 with `O_EXCL|O_NOFOLLOW`; audit files 0600. Add `.rela/` to
`.gitignore` (create or append).
5. **Open the target set** through a new exported, sqlite-tagged appbuild
function that returns store, validated state KV, comments store and migration
state store on one handle. `openBackend`/`backendServices` use the same
function, so a new sqlite table cannot be forgotten silently.
6. **Copy data** (`internal/backendcopy`) under
`store.WithAttribution{User: principal or system:fs-import, Tool: fs-import}`.
The target is the judge: every row is written, every row error is recorded
rather than stopping at the first; any error means the whole staging directory
is deleted and all errors are listed. Every row is also checked for `IsLocked()`
and attachments for the git-crypt header. Order: entities (all faces),
relations, attachments per family, comments (imported ids only), migration state
(`Save` of the whole `State`), state keys.
7. **Value normalization**: values are converted to what a write through the
API would store. A `time.Time` on a `date` property becomes `YYYY-MM-DD`, on a
`datetime` property RFC 3339; other values take their JSON round-trip form.
Values JSON cannot hold (`NaN`, `Inf`, non-string map keys) are row errors.
Normalized values are counted in the report.
8. **File reconciliation**: walk `entities/`, `relations/`, `attachments/`,
`.rela/comments/` and match every file against what was copied; list the rest
with a reason.
9. **Finish**: `ReconcileDerivedIndexes`; `PRAGMA wal_checkpoint(TRUNCATE)`;
close; assert no `-wal`/`-shm`; check source freshness is unchanged (else fail:
the source changed during the run); rename the staging directory to the target
(fails if the target appeared); reopen `rela.db` read-only and run verify; write
the audit record through `audit.NewFilesystem`. On failure after the rename:
report clearly and remove the target (it was created by this run).

`.rela/` classification (every entry falls in exactly one group; anything else
is listed in the report):
- config files, copied: `config.yaml`, `mail.yaml`, `ai.yaml`,
`secrets.yaml`, `audit/`;
- state keys, into `state_kv`: `caldav/aliases.json`, `user-defaults.yaml`,
`palette.yaml`, `theme/` (logo), `next-action-state.json`,
`scheduler-run-state.json` (last-run times only; in-flight runs dropped),
`migration/` (drift ledger, legacy marker);
- skipped: `comments/` (copied separately), `documents/` (render cache),
`search/`, `fsstore-index.json`, `pending-deletes.json`, `migration.lock`,
`scheduler-run-children/`.

`internal/backendcopy` takes the source through a narrow consumer-side interface
(`ListEntities`, `ListRelations`, `ListFamilyAttachments`,
`ReadFamilyAttachment`) and the target as `store.Store` (its `Tx` returns one).
The CLI command's `Run()` takes no injected services, like the other `db`
commands, so it never discovers or opens a project from the working directory.

Alternatives rejected:
- Revive `rela sync`: removed as unused, needs a running server, entities
and relations only.
- In place: leaves the fs data next to an unread database.
- Through entitymanager: runs automations during a move, refuses states fs
allows.
- A pre-scan that re-implements sqlite's rules: would drift from the store.
The target judges instead.
- Comparing `store.VersionOf` across backends: not comparable.

**Files to modify:**
- new `internal/backendcopy/` (copy core, normalization, verify, tests)
- new `internal/cli/db_importfs_sqlite.go`, `internal/cli/db.go`,
`internal/cli/db_nonpostgres.go`, `internal/cli/db_postgres.go`
- `internal/appbuild/appbuild_sqlite.go`, `configloader_sqlite.go` (exported
target opener), new fs read-only source opener
- `internal/store/fsstore` (index-cache bypass option), `internal/storage`
(read-only FS wrapper, if none exists)
- `internal/comments/filecomments` (exported thread-key listing)
- `internal/audit` (op constant)
- `.go-arch-lint.yml`, `.testcoverage.yml`
- `docs-project/entities/guides/GUIDE-sqlite-backend.md`,
`GUIDE-cli-reference.md`; regenerated `docs/`
- `internal/appbuild/statekv_nodb.go` (stale comment)
- `CLAUDE.md` (fifth raw-store exception)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Source and target paths (operator shell): see Approach step 1.
- Source content: read through fsstore, `filecomments`, the migration-state
bridge; the target store validates every row. `.rela/` entries by allowlist
classification. State keys pass through `state.ValidatedKV`.
- File copy: `os.OpenRoot`, regular files only, symlinked data files refused.

**Security-Sensitive Operations:**
- `secrets.yaml`: 0600, `O_EXCL|O_NOFOLLOW`, never widened, never printed.
`secrets.Load` is not called (it would read `CREDENTIALS_DIRECTORY`).
- `.rela/` covered by `.gitignore`, so `rela.db`, secrets and audit cannot be
committed by accident.
- Locked content refused at three points (pre-scan, write loop, verify).
- Trust boundary is the operator shell: no ACL check, explicit attribution,
one audit record, written also on failure.
- Plaintext warning when `.gitattributes` marks data paths `filter=git-crypt`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- `internal/backendcopy` unit tests use fsstore as the source (untagged) and
memstore as the target, over a fixture with unquoted dates, `2.0`, `.inf`, `{1:
a}`, case-colliding ids, the same id in two type folders, an undeclared type
folder, an unparsable file name, a mismatched relation file, an oversized
attachment, a named face without a default face.
- `internal/cli` integration test (sqlite tag) over the same fixture: AC1,
AC2 (tree hash), AC6, AC7, AC9, AC10.
- Guard test: list `sqlite_master` on a fresh database; fail on any table the
import has not marked as copied or excluded.
- Negative tests per AC3, AC4, AC5, AC8.
- Run the command from inside the source directory: no `.rela/rela.db`
appears there.

**Edge Cases:**
- Empty project: succeeds.
- Ids differing only in case: row error listing both.
- Entity files of an undeclared type: listed as not copied (decided with the
user).
- Frontmatter `id:` differing from the file name: the file name wins (fsstore
rule); listed as a warning.
- Unicode in ids, titles, file names; SQLite's `lower()` folds ASCII only, so
non-ASCII case pairs are accepted, matching the store.
- Large attachment within the cap; one over the 64 MiB cap is a row error.
- Comment thread whose entity is not imported: listed, not copied.
- Source modified during the run: run fails.
- Target created by someone else during the run: rename fails, staging
removed.

**Negative Tests:**
- Existing target, nested paths (also differing only in case), source
without `schema.yaml`, source with `rela.db`: refused, nothing written.
- Locked entity, locked attachment, lock appearing after the pre-scan.
- Store write error, leftover WAL, verification mismatch: nothing left.
- Non-sqlite build: clear error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Value normalization changes how values are stored (dates become strings).
Mitigation: rules follow what an API write stores; counted in the report;
canonical hash after normalization in verify.
- fsstore skips files silently. Mitigation: file-level reconciliation walk.
- Source changing during the run. Mitigation: freshness check at the end.
- Lost on purpose: original modification times and git history. Documented.
- On first open, the version sweep records a baseline version per row (about
5 minutes later) and search reindexes. Documented.
- Effort grew from the first estimate.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist created (DOCS-E8LVR2)

**Documentation Impact:**
- [x] docs/cli-reference.md - New/changed commands
- [x] docs/sqlite-backend.md - "Migrating between backends", and the stale
runtime-state section
- [x] CLAUDE.md - fifth raw-store exception

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review responses linked to TKT-YNKKRQ.
