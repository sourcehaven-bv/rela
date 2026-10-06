---
id: PLAN-AHAPKV
type: planning-checklist
title: 'Planning: Boot a SQLite project from its database: schema and acl.yaml through the layered config loader'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Recorded after implementation: the work landed on feat/self-contained-sqlite
(39b8e316, 72a7c8cb) before this checklist was filled in. Each item states what
was done.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: the sqlite recipe opens the database before `prepare()`; a disk-first
layered `config.Loader` over it (`layerProjectConfig`); `schema.yaml` with its
includes and the migration check, and `acl.yaml`, read through that loader
(`loadMetamodel`, `readACLPolicy`); one database handle shared by the config
loader, the store and the runtime-state overrides; the loader carried on
`Config` so the schema hot-reload path reads through the same source.

Out: data-entry, scripts, templates, `custom/` and `apps/` (TKT-3RDMDD); `rela
db load --data` / `dump --data` (TKT-LWOCW9). The config half of `rela db load`
/ `rela db dump` landed in 39b8e316 and is tracked under TKT-LWOCW9.

**Acceptance Criteria:**

1. A directory with only `.rela/rela.db` (config in `project_files`) boots.
Test: `TestSQLite_BootsFromBakedConfig`, `TestSQLite_BakedACLIsRead`
(`internal/appbuild/projectfiles_sqlite_test.go`). Discovery by the `.rela/`
marker was already supported (`internal/project/context_test.go`, "finds project
by .rela directory").
2. Disk still wins when both exist. Test: `TestSQLite_DiskConfigShadowsBaked`;
`TestLayered_Load_PrimaryWins` (`internal/config/layered_test.go`).
3. fs/memory/postgres builds unchanged: with no recipe-supplied loader,
`readACLPolicy` and `loadMetamodel` take the previous disk path. Test: the
existing appbuild and CLI suites in `just test`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach recorded in DEC-R9M57Z)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal wiring change; no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal wiring change)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A; see DEC-R9M57Z.

**Existing Solutions:**

- `config.Layered` and `configsql.Loader` already existed; this ticket wires
them into the boot path.
- `metamodel.NewFSLoader` takes an `fs`; `config.NewStorageFS` gives it a
read-only view of the loader, so the metamodel loader is unchanged.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `appbuild_sqlite.go` calls `openDatabase` first, sets
the unexported `Config.projectConfig` to `layerProjectConfig(...)`, then runs
`prepare()`. `acl.ParsePolicy` was added so a policy read from bytes names its
source in errors. The handle is passed on to `openBackend` and
`backendServices`.

Alternative rejected: keeping `projectConfig` on `backendOverrides`. It is read
in `prepare()`, before the overrides exist, and a successor `SharedBase` built
for hot reload would not see it.

**Files to modify:** `internal/appbuild/appbuild.go`, `appbuild_sqlite.go`,
`configloader_sqlite.go`, `derivedschema_sqlite.go`, `projectfiles_sqlite.go`;
`internal/acl/policy.go`; `internal/config/storagefs.go`;
`internal/config/configsql/configsql.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Config rows from `project_files`: `configsql` rejects unsafe names
(`TestLoader_RejectsUnsafeNames`).
- `config.StorageFS` treats paths outside the root as absent and is read-only
(`TestStorageFS_OutsideRootIsAbsent`, `TestStorageFS_IsReadOnly`).
- Collecting config from disk refuses symlinks (72a7c8cb,
`TestSQLite_CollectRefusesSymlinks`).

**Security-Sensitive Operations:**

- `acl.yaml` may now come from the database. A broken baked policy fails the
boot rather than falling back to no ACL (`TestSQLite_BakedACLIsRead`). Only a
genuinely absent file selects NopACL, as before.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above. The appbuild tests boot
full `Services` through `appbuild.Discover`.

**Edge Cases:** legacy `metamodel.yaml` stored as `schema.yaml`
(`TestSQLite_CollectProjectConfig`); a project found by its schema with no
`.rela/` yet (`openDatabase` creates it); database open fails after the loader
is built (handle closed on every error path).

**Negative Tests:** broken baked `acl.yaml` fails the boot; symlinks refused on
collect; nil layers rejected by `NewLayered` (`TestNewLayered_RejectsNil`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Changing `prepare()` could alter the other builds. Mitigated: the old path is
kept verbatim when `projectConfig` is nil.
- The hot-reload path has no dedicated test against a database-only project;
it relies on `projectConfig` living on `Config`.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** CLAUDE.md storage section (7f2902d6),
`docs/sqlite-backend.md` and GUIDE-sqlite-backend.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design settled in DEC-R9M57Z; the code was covered by the branch-wide review recorded on TKT-FGIWPE)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none specific to this ticket.
