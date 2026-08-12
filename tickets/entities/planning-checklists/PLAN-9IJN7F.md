---
id: PLAN-9IJN7F
type: planning-checklist
title: 'Planning: Rename metamodel.yaml to schema.yaml with backward-compatible dual-name discovery'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** rela has two names for one concept. The external surface already
says "schema" (`rela schema`, `/api/v1/_schema`, and `kong.go:90`'s help text
`"View the metamodel schema."` which glosses the term); only the filename, the
Go package, and the MCP tool `get_metamodel` still say metamodel. The filename
is the last user-facing holdout.

**Scope — IS in scope:**

- Dual-name discovery in `internal/project.Discover()` (`schema.yaml` preferred, `metamodel.yaml` accepted).
- `Context.MetamodelPath` → `SchemaPath`, carrying the *resolved* path.
- One-shot deprecation warning **at process startup in every entry point**, driven by a `Context` field (revised per RR-9XXI80).
- `rela migrate` performs the file rename.
- New projects (`rela init`, desktop new-project) create `schema.yaml`, **with a widened already-initialized guard** (per RR-NRFBZE).
- MCP `get_metamodel` → `get_schema` with the old name retained as an alias.
- Desktop app project detection + welcome-screen copy.
- `internal/docscapture` config file list.
- Docs, `CLAUDE.md`, and in-repo project files (`tickets/`, `docs-project/`, `examples/`, `prototypes/`).

**Scope — explicitly NOT in scope:**

- Renaming the Go package `internal/metamodel` (internal precision is fine; large churn; separate ticket).
- Moving `automations:` out to its own file (would make "schema" precisely accurate, but deferred).
- Renaming the `Metamodel` Go type, `metamodel.NewFSLoader`, or any internal identifier not on the discovery path.
- Removing `metamodel.yaml` support (deprecation only; removal waits for a major version).
- Changing the `.rela/` marker's precedence in `Discover()` (see AC-4 / RR-JANQG7 — status quo documented, not altered).

**Acceptance Criteria:**

1. A project containing only `schema.yaml` is discovered and loads normally, with no deprecation warning.
2. A project containing only `metamodel.yaml` is discovered and loads normally, and emits exactly one deprecation warning naming `rela migrate`.
3. A project containing both files loads `schema.yaml` and warns that `metamodel.yaml` is being ignored.
4. A project containing neither, but with a `.rela/` directory, is still discovered (existing behaviour preserved); the missing-file error names `schema.yaml`, mentions the legacy name is accepted, **and names the directory that was selected as the root** so a stray `.rela/` shadowing a real parent project is diagnosable (RR-JANQG7).
5. `rela rename-type` on a legacy project rewrites `metamodel.yaml` in place — it does NOT create a stray `schema.yaml`.
6. `rela migrate` on a legacy project renames the file to `schema.yaml`, **applies content migrations to the renamed file in the same run**, and reports both. A second run is a no-op.
7. `rela init` creates `schema.yaml`. **In a directory containing EITHER name it refuses**, with a message distinguishing "already initialized" from "legacy project — run `rela migrate`" (RR-NRFBZE).
8. MCP clients calling `get_metamodel` still work; `get_schema` is present and equivalent.
9. The deprecation warning appears once per process at startup — in the CLI, `rela-server`, MCP, and desktop — and NOT per-request or per-tool-call.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — no external unknowns; this is a rename with a
compatibility shim, fully scoped by direct codebase investigation (documented
below).

**Existing Solutions:**

- **Libraries:** none applicable. This is filesystem-name resolution, ~15 lines.
- **Prior art in codebase — the `.rela` legacy marker.** `internal/project/context.go:62-66`
already implements a "legacy/alternative marker" fallback in the same
`Discover()` loop. The new dual-name check follows that established shape rather
than inventing a mechanism.
- **Prior art — optional-file skip.** `internal/projectsetup/migrate.go:105-108`
skips files that don't exist, with a load-bearing comment (RR-SVQ5HE) about
`acl.yaml` absence being meaningful. Same discipline applies here: absence of
`schema.yaml` must not be conflated with absence of a project.
- **Reference implementations:** the standard ecosystem pattern for config renames
is try-new-then-fall-back-to-old with a deprecation notice (e.g. `.eslintrc` →
`eslint.config.js`, `.babelrc` → `babel.config.js`). Nothing exotic needed.
- **Concepts reviewed:** `project-layout` (created for this ticket — nothing
previously owned `internal/project`), `mcp-api` (tool rename), `metamodel-types`
(rejected as primary: it covers *property types*, not file naming).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`internal/project/context.go`** — add constants:

   ```go
   SchemaFile          = "schema.yaml"
   LegacyMetamodelFile = "metamodel.yaml"
   ```

In the `Discover()` walk, at each directory level stat `schema.yaml` first, then
`metamodel.yaml`. Whichever is found determines the root AND is recorded.
Checking both names at each level (rather than two full walks) preserves
"nearest project root wins" semantics. The existing `.rela/` branch keeps its
current position and precedence — see RR-JANQG7 under Edge Cases.

2. **`Context` gains resolved state**, not a boolean:

   ```go
   SchemaPath     string // resolved path to whichever file was found
   SchemaIsLegacy bool   // true when the resolved file is metamodel.yaml
   ```

`SchemaPath` is the empty-state-safe field: when a project is discovered via the
`.rela` marker with neither file present, `SchemaPath` points at the *preferred*
new name (`schema.yaml`) so downstream "file missing" errors report the name
users should create.

3. **In-place writers are correct by construction.** `renametype.go:81` and
`projectsetup/migrate.go:145` already consume `ctx.MetamodelPath`; renaming the
field to `SchemaPath` keeps them writing to whichever file was resolved. This is
the reason for storing a path rather than recomputing from a constant.

4. **Deprecation warning once per process at startup** (revised per RR-9XXI80).
Driven by `ctx.SchemaIsLegacy`, guarded by a `sync.Once`, emitted where each
binary opens its project — the shared `appbuild.Discover` path
(`appbuild.go:688`) plus the desktop app's direct `project.Discover` call
(`main.go:797`). NOT inside `project.Discover()` itself, which runs per-request
in server contexts. Attaching it only to the CLI command entry would leave
server-only, MCP-only, and desktop-only users never warned before the legacy
name is dropped.

5. **`rela migrate` renames the file.** NOTE — the `migration.Migration` interface
operates on `yaml.Node` trees (content transforms) and cannot express a file
rename, so the rename is a distinct step in `projectsetup.Migrate`, not a
registered `Migration`. **Ordering is load-bearing** (RR-ZB1FJB): the rename
runs FIRST, then `ctx.SchemaPath` is updated, and only then is
`getMigrateFiles(ctx)` evaluated — otherwise the loop's stat-guard
(`migrate.go:106`) silently skips the now-missing old path and the project is
renamed but never content-migrated. **Refuse, don't skip, when the target
exists:** `storage.FS.Rename` wraps `os.Rename`, which silently replaces an
existing target on POSIX, so a stat-then-rename guard is advisory only. If
`schema.yaml` already exists alongside `metamodel.yaml`, return an actionable
error rather than renaming or silently continuing.

6. **MCP alias** — register `get_schema` and keep `get_metamodel` dispatching to
the same handler, since clients cache tool names in config.

7. **Desktop app** — `cmd/rela-desktop/main.go` uses `project.Discover` for
*opening* (line 797), so that path is covered. But it has three independent
literal checks that must be updated separately: the recursive project scan (line
459), the "is this a project dir" check (line 785), and new-project creation
(line 424 — which has **no existence guard at all**, RR-NRFBZE). Welcome-screen
copy at `welcome.go:162` too.

8. **`internal/docscapture/server.go:143`** has its own config-file list that
must accept both names.

9. **`rela init` guard must stat both names** (RR-NRFBZE).
`projectsetup/init.go:43-47` currently errors only if the single computed path
exists. Repointing that path to `schema.yaml` without widening the guard means
`rela init` in a legacy project sees no `schema.yaml`, believes the directory is
uninitialised, and writes a **default** `schema.yaml` next to the user's real
`metamodel.yaml`. Discovery then prefers the default and the project comes up
with an empty schema — silent user data loss. Guard on either name; give
distinct messages for "already initialized" and "legacy project".

**Alternatives considered:**

- *Error when both files present* — rejected. If both exist the user is
mid-migration and `schema.yaml` is the file they just wrote; preferring it does
the right thing, while erroring blocks a user whose intent is unambiguous. The
stale-schema risk is covered by the warning. (Note the deliberate asymmetry with
`init`/`migrate`, which DO refuse — those are *writes*, where guessing wrong
destroys data; discovery is a *read*, where guessing wrong is recoverable.)
- *Register the rename as a `migration.Migration`* — rejected; the interface is
`yaml.Node`-based and structurally cannot rename a file.
- *Symlink/copy compatibility* — rejected; two real files diverge silently.
- *Boolean `UsedLegacyName` flag with path recomputed from the constant* —
rejected; that is precisely the bug that would make `rename-type` write to the
wrong file.
- *Changing `.rela/` marker precedence* — rejected as scope creep; documented in
AC-4 instead (RR-JANQG7).

**Files to modify:**

| File | Change |
|---|---|
| `internal/project/context.go` | dual-name constants, `Discover()` fallback, `SchemaPath`/`SchemaIsLegacy` |
| `internal/appbuild/appbuild.go:688,751` | field rename + startup deprecation warning |
| `internal/projectsetup/migrate.go` | rename step (ordered before `getMigrateFiles`), refuse-on-existing-target, resolved basename |
| `internal/projectsetup/init.go:43-47` | **widen already-initialized guard to both names** |
| `internal/projectsetup/validate.go`, `app.go` | field rename |
| `internal/renametype/renametype.go:81` | field rename (behaviour correct via resolved path) |
| `internal/cli/init.go:25` | output message says `schema.yaml` |
| `internal/mcp/tools.go`, `dispatch*.go`, `server.go:13` (stale doc) | `get_schema` + alias |
| `internal/dataentry/commands.go:323` | `Metamodel: "metamodel.yaml"` literal |
| `internal/docscapture/server.go:143` | config file list |
| `cmd/rela-desktop/main.go:424,459,785,797`, `welcome.go:162` | independent detection, **new-project guard**, copy |
| `internal/config/config.go:3`, `internal/dataentry/handlers.go:197-200` | doc comments |
| `internal/metamodel/errors.go`, `internal/errors/errors.go` | error message wording |
| docs, `CLAUDE.md`, `tickets/`, `docs-project/`, `examples/`, `prototypes/` | rename files + references |

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Filenames are compile-time constants**, never user-supplied. No traversal
surface is added: `Discover()` joins fixed basenames onto directories it is
already walking. This is an allowlist of exactly two names.
- **Project root** comes from `--project`/cwd as before; unchanged.
- Invalid/absent input: unchanged `ErrNoProject` behaviour, with improved wording.

**Security-Sensitive Operations:**

- **File rename during `rela migrate`** — the only new write. `storage.FS.Rename`
is an `os.Rename` wrapper and therefore **replaces an existing target atomically
and silently on POSIX**; the stat-first guard is advisory, not a lock, so a
TOCTOU window exists (RR-ZB1FJB). Mitigated by refusing the whole operation when
the target exists rather than proceeding. Trust boundary is the operator's
shell, same as `db migrate`.
- **`rela init` / desktop new-project writes** — the guard widening in RR-NRFBZE
is a *data-integrity* control, not a confidentiality one, but it is the
highest-impact safety item in this ticket.
- **No confidentiality impact.** Per `CLAUDE.md` § "The configuration is not a
secret; the data is", the schema filename and its contents are operator-authored
repo files and already disclosed. Nothing here touches entity/relation content,
entity existence, ACL gating, or secrets.
- **Error messages** name only fixed config filenames and the project root the
caller already supplied — no credentials, no paths outside the caller's own
tree.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (mapped to acceptance criteria):**

| AC | Test | Location |
|---|---|---|
| 1 | `Discover` on dir with only `schema.yaml` → found, `SchemaIsLegacy=false` | `internal/project/context_test.go` |
| 2 | `Discover` on dir with only `metamodel.yaml` → found, `SchemaIsLegacy=true`, `SchemaPath` ends in `metamodel.yaml` | `internal/project/context_test.go` |
| 3 | both present → `SchemaPath` ends in `schema.yaml`, `SchemaIsLegacy=false` | `internal/project/context_test.go` |
| 4 | only `.rela/` → discovered, `SchemaPath` ends in `schema.yaml`; error text names the selected root | `internal/project/context_test.go` |
| 5 | **regression, load-bearing:** `rename-type` in a legacy project mutates `metamodel.yaml` and creates no `schema.yaml` | `internal/renametype/renametype_test.go` |
| 6 | `Migrate` renames legacy file **and applies content migrations to it in the same run**; second run is a no-op | `internal/projectsetup/migrate_test.go` |
| 7 | **`init` refuses in a dir with only `metamodel.yaml`** and does not write `schema.yaml`; refuses with `schema.yaml`; succeeds in an empty dir | `internal/projectsetup/init_test.go` |
| 8 | both `get_metamodel` and `get_schema` dispatch | `internal/mcp/dispatch_test.go` |
| 9 | warning emitted once across repeated `Discover` calls in one process | `internal/appbuild/` |

**Integration approach:** the existing `internal/cli` command tests and
`appbuildtest` fixture exercise the full discovery→load→command path. Add a
legacy-named-project fixture so at least one end-to-end CLI test runs entirely
against `metamodel.yaml`, proving the compat path works beyond unit level.
`e2e/` scenarios stay on the new name.

**Edge Cases:**

- **`init` in a legacy project** (RR-NRFBZE) — must refuse; the highest-severity
case, since the failure is silent data loss rather than an error.
- **`migrate` ordering** (RR-ZB1FJB) — rename-then-migrate must apply content
migrations to the renamed file; assert both effects in one run, not just the
rename.
- **`migrate` when `schema.yaml` already exists** — refuse with an actionable
error; must NOT overwrite (POSIX rename replaces silently).
- **Stray `.rela/` shadowing a parent project** (RR-JANQG7) — a subdirectory with
`.rela/` but no schema file is selected as root and the walk stops; the parent's
real schema is never found. Status quo, now explicitly tested and covered by a
diagnosable error message.
- Both files present, `schema.yaml` malformed → parse error surfaces from
`schema.yaml`; must NOT silently fall back to `metamodel.yaml` (falling back
would load a different schema than the user is editing). Explicit test.
- Nested projects: child has `metamodel.yaml`, parent has `schema.yaml` → child
wins (nearest root), guaranteed by checking both names per directory level.
- `schema.yaml` present but unreadable (permissions) → error, no fallback.
- Empty-state: neither file, no `.rela` → `ErrNoProject`, unchanged.
- Symlinked `schema.yaml` → `Stat` follows the link; existing behaviour, unchanged.

**Negative Tests:**

- Malformed `schema.yaml` must produce a metamodel parse error, NOT `ErrNoProject`
— already pinned by `internal/cli/mcp_wiring_test.go:64`. That assertion must
keep passing.
- Directory named `schema.yaml` (not a file) → must not be treated as a project marker.
- `init` must not partially initialise on refusal (no stray `entities/` dirs left behind).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| `rela init` in a legacy project writes a default schema and silently shadows the real one | **Critical** | RR-NRFBZE: widen guard to both names; AC-7 test asserts refusal + no file written |
| `migrate` renames but silently skips content migrations (stat-guard on stale path) | **High** | RR-ZB1FJB: rename first, update `SchemaPath`, then build the file list; AC-6 asserts both effects |
| A writer recomputes the path from `SchemaFile` and clobbers a legacy project | **High** | Store the resolved path on `Context`; AC-5 regression test pins `rename-type` |
| Desktop app has 3 independent literal checks bypassing `Discover()` | **Medium** | Enumerated in the file table; legacy projects would silently vanish from the picker if missed |
| Server/MCP/desktop-only users never see the deprecation notice | **Medium** | RR-9XXI80: warn once per process at startup in every entry point, not just CLI |
| Fallback masks a malformed `schema.yaml` | **Medium** | Fallback is on *absence* only, never on parse failure; explicit edge-case test |
| Stray `.rela/` shadows a real parent project | **Low** | RR-JANQG7: status quo preserved; error names the selected root so it is diagnosable |
| Large mechanical diff obscures a real change | **Low** | Only 16 non-test constant usages; split doc/fixture renames from logic changes in review |

**Effort:** m — small logic surface, wide mechanical surface. The three
write-path guards (init, migrate, rename-type) carry essentially all the risk.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** This is user-facing (a file every user has), so docs
are mandatory, not optional:

- [x] `docs/metamodel.md` — the file's own documentation; rename references, add a deprecation note
- [x] `docs/cli-reference.md` — `rela migrate` gains the rename behaviour; `rela init` output and refusal messages change
- [x] `CLAUDE.md` — "Project files" tree block and the config-is-not-a-secret list
- [x] `README.md` — any quickstart mentioning `metamodel.yaml`
- [ ] `docs/data-entry.md` — N/A, no UI change
- Plus a migration/upgrade note stating both names work and when the legacy one will be dropped.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **RR-NRFBZE** (critical) — `rela init` clobbers an existing legacy project. Addressed: approach step 9, AC-7, edge case, risk table.
- **RR-JANQG7** (significant) — `.rela/` marker preempts the parent-directory fallback; the plan's nested-project claim was incomplete. Addressed: AC-4, edge case, out-of-scope note.
- **RR-ZB1FJB** (significant) — `migrate` rename TOCTOU + unspecified ordering vs content migrations. Addressed: approach step 5, AC-6, security section.
- **RR-9XXI80** (minor) — deprecation warning unreachable for server/MCP/desktop users. Addressed: approach step 4, AC-9.
