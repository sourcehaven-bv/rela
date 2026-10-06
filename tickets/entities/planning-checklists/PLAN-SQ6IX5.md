---
id: PLAN-SQ6IX5
type: planning-checklist
title: 'Planning: Data-entry, templates and scripts read config through the services'' config loader'
started: "2026-10-04"
completed: "2026-10-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

Recorded after implementation: the work landed on feat/self-contained-sqlite
(44e2e0d8, 6a6e8b42, a9b571c2) before this checklist was filled in. Each item
states what was done.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: `dataentry.NewApp` takes the services' config loader instead of building its
own `config.FSLoader` (data-entry.yaml, action, document and export-render
script checks); `lua.ReadDeps.Files` and `ReadDeps.ReadScript` for `scripts/`,
`actions/` and `validations/`, used by the script engine, actions, document
lists, Lua validations and the MCP Lua tool; `custom/` and `apps/` served
through `Services.ProjectFiles` with the `Stat` and `Dirs` capabilities the
handlers need (`config.Stater`, `config.DirLister`, implemented by `configsql`,
`Layered` and `rootfs`); entity templates read through the project config
(`appbuild/templatefs.go`, `config.StorageFS`); the new `internal/rootfs` disk
layer with `os.Root` containment.

Out: the boot-time schema and `acl.yaml` read (TKT-WFB1YH); `rela db load` /
`dump` (TKT-LWOCW9).

**Acceptance Criteria:**

1. A project with config only in rela.db serves the data-entry app, runs
actions, validations and scheduled scripts, applies entity templates and serves
`custom/` and `apps/`. Tests (`internal/appbuild/projectfiles_sqlite_test.go`):
`TestSQLite_BakedScriptsAreRead` (the `ReadScript` seam every script reader
uses), `TestSQLite_BakedAssetsAreServed`, `TestSQLite_BakedTemplatesAreUsed`.
The scheduler reads `schedules.yaml` through `ws.Config()` and its scripts
through the same `ReadScript`. Automations are declared in the schema, which
TKT-WFB1YH reads through the loader. No single test runs an automation, an
action and a scheduled task against a database-only project; the shared seam is
tested instead.
2. Path containment is preserved on both backends. Disk: `TestDir_Load`,
`TestDir_NestedRootRefusesSymlinks`, `TestDir_SymlinkStaysInItsDirectory`
(`internal/rootfs`); `TestOpenCustomEntry_NeverEscapes`,
`TestHandleCustomAsset_TraversalNeverEscapes`, `TestOpenAppEntry_Traversal`
(`internal/dataentry`); `TestReadDeps_ReadScriptPaths`,
`TestEngine_ExecuteFile_PathTraversal`, `TestExecuteAction_PathTraversal`.
Database: `TestLoader_RejectsUnsafeNames`, `TestStorageFS_OutsideRootIsAbsent`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach recorded in DEC-R9M57Z)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal refactor onto an existing interface)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal refactor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A; see DEC-R9M57Z.

**Existing Solutions:**

- `config.Loader` already served data-entry.yaml on the services side; this
ticket makes every operator-file reader use it.
- The per-reader `os.OpenRoot` code in `script`, `validation` and `dataentry`
was folded into `internal/rootfs`, which keeps the same containment.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** consumers declare the narrow interface they need at the
call site (`lua.ProjectFiles` with one `Load` method; `config.Stater` and
`config.DirLister` as optional capabilities). The wiring site passes
`Services.ProjectFiles()`. `lua` falls back to `rootfs` when `Files` is nil, so
it does not depend on `config` (arch-lint rule added for `rootfs`).

Alternative rejected: a `FSLoader` over the project root as the disk layer. It
does not stop a symlink from leaving its directory, which the old `os.Root`
readers did.

**Files to modify:**
`internal/dataentry/{app,apps,apps_handler,custom,custom_handler,mailgate,router,watcher}.go`;
`internal/lua/deps.go`; `internal/script/{action,executor,list_document}.go`;
`internal/validation/lua.go`; `internal/mcp/tools_lua.go`;
`internal/config/{config,layered,storagefs}.go`, `configsql/configsql.go`;
`internal/rootfs/rootfs.go`; `internal/appbuild/{appbuild,templatefs}.go`;
`cmd/rela-server`, `cmd/rela-desktop`, `internal/docscapture` wiring;
`.go-arch-lint.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Request paths for `custom/` and `apps/`: validated (`validCustomEntry`,
traversal tests above), then read through the loader.
- Script paths from config and MCP: `ReadScript` requires a local path ending
in `.lua`.
- Names stored in `project_files`: `configsql` rejects unsafe names.

**Security-Sensitive Operations:**

- File reads under the project root: `rootfs` nests an `os.Root` per area, so
a symlink cannot leave its directory.
- `ReadScript` errors name the script, never a filesystem path, so they are
safe to return to HTTP and MCP callers.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria above. The appbuild tests boot
full `Services` with the files removed from disk.

**Edge Cases:** empty custom file served; directories and oversize files; an
absent `apps/` directory; a symlink inside the project; a template with a
variant (`doc--short.md`); hidden files.

**Negative Tests:** traversal and absolute paths refused; wrong script extension
refused; symlink escapes refused. A FIFO is refused by `rootfs` since the
TKT-FGIWPE follow-up (e8924a08, `TestDir_RefusesNonRegularFiles`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Replacing the per-reader `os.OpenRoot` code could weaken containment.
Mitigated by the traversal and symlink tests above; per-area containment was
tightened in a follow-up under TKT-FGIWPE (e8924a08, `TestDir_AreaContainment`).
- `dataentry.NewApp` now requires the loader. It rejects nil with an error;
no test pins that rejection.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** CLAUDE.md storage section (7f2902d6) names every
reader and the `rootfs` rule; `docs/sqlite-backend.md` and GUIDE-sqlite-backend
list what the database can carry.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design settled in DEC-R9M57Z; the code was covered by the branch-wide review recorded on TKT-FGIWPE)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-BNQKEW (redundant `..` guard in `rootfs`, won't
fix) is the only finding in this ticket's code.
