---
id: PLAN-M1RA
type: planning-checklist
title: 'Planning: Migrate CLI to scoped services helper (drop package globals)'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `internal/cli/root.go` holds package-level globals populated by
`PersistentPreRunE`:

```go
var (
    ws         *workspace.Workspace      // god-object reference
    projectCtx *project.Context           // derived from ws.Paths()
    meta       *metamodel.Metamodel       // derived from ws.Meta()
    out        *output.Writer
)
```

~30 CLI subcommands reach for `ws.X()` directly. The 22 distinct methods called
span:

- Read services: Store, Meta, Paths, Tracer, Config, FS, EntityManager, LuaCache, LuaWriteDeps, Templater (also Searcher implicitly via EntityManager)
- CLI-specific facades (~700 LOC in workspace): AnalyzeAll, CheckCardinality, FindDuplicates, FindGaps, FindOrphansWithScope, FindOrphanedTempFiles, CleanupOrphanedTempFiles, RunValidations, RunValidationsFiltered, RenameEntityType, AttachFile, ListAttachments
- Plus: ResolveEntityType (helper)

After this ticket, `internal/cli` no longer imports `internal/workspace` in
subcommand files. Subcommands consume services via the cobra command context,
using purpose-scoped bundles (read / write / analyze) — each subcommand declares
which bundle it needs. The wiring helper still constructs a
`*workspace.Workspace` and holds it inside the analyze/attach/rename facade
fields (transitional — TKT-2W0X lifts those into dedicated packages).

**Design-review pushback addressed (round 1):**

The first draft proposed a single `cliServices` bundle exposed as a
package-level global `svc`. Cranky review (C1 + C2) flagged this as:
- a service locator (one grab-bag struct handed to every consumer,
hiding what each subcommand actually needs)
- a rename of the existing `ws` global, not a refactor

This revision adopts mid-grain purpose-scoped bundles + cobra-context passing
per the review's "do it right now" recommendation. Resolves both findings in-PR;
closes the workspace-decomposition arc cleanly without stacking transitional
cleanup tickets.

**Scope (in):**

- New `internal/cli/cli_wiring.go`:
  - `cliRead` bundle exposing read-only services (Store, Meta, Paths, Tracer, Searcher, Config, Templater, FS — 8 methods). Used by read-only subcommands and consumed by all bundles below.
  - `cliWrite` bundle: embeds `cliRead` + adds EntityManager, Validator, LuaCache, LuaWriteDeps (4 more methods). Used by write-path subcommands.
  - `cliAnalyze` bundle: embeds `cliRead` + adds AnalyzeAll, CheckCardinality, FindDuplicates, FindGaps, FindOrphansWithScope, FindOrphanedTempFiles, CleanupOrphanedTempFiles, RunValidations, RunValidationsFiltered, RenameEntityType, AttachFile, ListAttachments (12 facade forwarders that hold a `*workspace.Workspace`). Used by analyze/gc/validate/rename/attach subcommands.
  - `newCLIServices(startDir)` constructs all three from a single shared workspace instance.
  - Compile-time `var _ cliRead = (*cliReadImpl)(nil)` etc. assertions.
- Cobra-context plumbing:
  - `cliReadFromContext(ctx) cliRead`, `cliWriteFromContext(ctx) cliWrite`, `cliAnalyzeFromContext(ctx) cliAnalyze` accessors.
  - `PersistentPreRunE` constructs the bundles via `newCLIServices`, stashes them on `cmd.SetContext(...)` so subcommand `RunE` retrieves them.
  - No package-level service globals. `out` stays as a package global (CLI output formatting; not workspace-related).
- Each subcommand declares which bundle(s) it needs:
  - Read-only commands (`show`, `list`, `trace`, `graph`, `export`, `template`, `fmt`): `cliRead`
  - Write-path commands (`create`, `delete`, `update`, `link`, `unlink`, `detach`, `import`, `normalize`, `script`): `cliWrite`
  - Facade commands (`analyze`, `gc`, `validate`, `rename`, `attach`, `attachments`): `cliAnalyze`
- `cli/test_helpers_test.go::storeSeeder.build()` returns a struct that satisfies the bundles. Subcommand tests get a constructed bundle (not a package global).
- After this PR, CLI tests can use `t.Parallel()` (today blocked by the `ws` package global). Marking parallel is OUT OF SCOPE; just enabling it is a follow-up nit.

**Scope (out):**

- **Lifting workspace facade methods** to dedicated packages — that's TKT-2W0X. The `cliAnalyze` bundle is the transitional shape; TKT-2W0X swaps its implementation without touching subcommands.
- Changes to subcommand argument parsing, flag schema, output format.
- `internal/cli/migrate.go`, `internal/cli/validate.go` use of free functions `workspace.DetectMigrations`, `workspace.Migrate`, `workspace.Validate` — these take a project root path, not a `*Workspace`. The plan keeps them as workspace-package functions. The free-function call sites mean these files still `import workspace`; acceptable transitional state.
- `internal/cli/flow.go`, `internal/cli/scheduler.go` use `workspace.Discover` for their own one-off workspaces (separate from the global). Keep as-is for this ticket.
- `out` package global (CLI output writer).
- Adding `t.Parallel()` to CLI tests. Bundle migration removes the blocker; actually doing it is a separate ticket-or-PR.

**Acceptance Criteria:**

1. **Package globals `ws`, `projectCtx`, `meta` are gone** from `internal/cli/root.go`. Verified by `grep -n '^var ws \|^var projectCtx\|^var meta ' internal/cli/root.go` → zero.
2. Three bundle interfaces in `cli_wiring.go`: `cliRead` (8 methods), `cliWrite` (cliRead + 4), `cliAnalyze` (cliRead + 12 facades). Each has a compile-time `var _ X = (*xImpl)(nil)` assertion.
3. **Each subcommand reads its bundle from `cmd.Context()`** via one of `cliReadFromContext`/`cliWriteFromContext`/`cliAnalyzeFromContext`. Verified by `grep -n 'FromContext(' internal/cli/*.go` showing the migration.
4. `cli/test_helpers_test.go::storeSeeder.build()` returns a bundle (not `*workspace.Workspace`). Tests still pass.
5. `go test -race ./internal/cli/...` passes.
6. `just lint`, `just arch-lint`, `just ci` pass.
7. `internal/cli/*.go` (excluding `cli_wiring.go`, `mcp.go`, `mcp_wiring.go`, `flow.go`, `scheduler.go`, `migrate.go`, `validate.go`) no longer imports `internal/workspace`. Verified by grep.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Reference implementations in repo:**

- `internal/cli/mcp_wiring.go` (PR #722, merged) — canonical wiring-helper pattern. CLI's helper reuses the same shape.
- `internal/mcp/test_helpers_test.go` (PR #722) — `newTestServices` stub pattern. CLI's `test_helpers_test.go` mirrors this.
- `internal/lua/deps.go` — `ReadDeps` / `WriteDeps` capability bundles split by read vs. write. The model for the cliRead/cliWrite split.
- `internal/scheduler/scheduler.go` — `WorkspaceProvider` 4-method consumer-side interface. Reference for "consumer-side interfaces at the call site."
- `cobra.Command.SetContext` / `cmd.Context()` — standard cobra plumbing for request-scoped state.

**Survey findings (count of `ws.X()` call sites by method, ranked):**

| Method | Approx callers | Bundle |
|---|---|---|
| ws.EntityManager | 12+ | cliWrite |
| ws.Meta | 18+ | cliRead |
| ws.Store | 12+ | cliRead |
| ws.Tracer | 8+ | cliRead |
| ws.Paths | 6+ | cliRead |
| ws.FS | 4+ | cliRead |
| ws.Config | 3 | cliRead |
| ws.Templater | 3 | cliRead |
| ws.LuaCache | 2 | cliWrite |
| ws.LuaWriteDeps | 1 | cliWrite |
| ws.AnalyzeAll | 1 | cliAnalyze |
| ws.CheckCardinality | 1 | cliAnalyze |
| ws.FindDuplicates | 1 | cliAnalyze |
| ws.FindGaps | 1 | cliAnalyze |
| ws.FindOrphansWithScope | 2 (analyze.go, gc.go) | cliAnalyze |
| ws.FindOrphanedTempFiles | 1 | cliAnalyze |
| ws.CleanupOrphanedTempFiles | 1 | cliAnalyze |
| ws.RunValidations | 1 | cliAnalyze |
| ws.RenameEntityType | 1 | cliAnalyze |
| ws.AttachFile | 1 | cliAnalyze |
| ws.ListAttachments | 1 | cliAnalyze |
| ws.ResolveEntityType | varies | free function (see M4 below) |

**Validator usage** is via `ws.Validator()` (not in the grep above because the
call is via the facades). It's exposed on cliWrite for write-path validation and
on cliAnalyze for batch validation.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Step 1 — Define the bundle interfaces

```go
// internal/cli/cli_wiring.go

// cliRead exposes read-only services. Most read commands consume only this.
type cliRead interface {
    Store() store.Store
    Meta() *metamodel.Metamodel
    Paths() *project.Context
    Tracer() tracer.Tracer
    Searcher() search.Searcher
    Config() config.Loader
    Templater() templating.Templater
    FS() storage.FS
}

// cliWrite is cliRead + write-path services. Embeds the read bundle.
type cliWrite interface {
    cliRead
    EntityManager() entitymanager.EntityManager
    Validator() validator.Validator
    LuaCache() *lua.Cache
    LuaWriteDeps() lua.WriteDeps
}

// cliAnalyze is cliRead + CLI-specific facade methods that today live
// on *workspace.Workspace. TKT-2W0X dissolves the facade by lifting
// these methods into dedicated packages (internal/analysis,
// internal/attachment, internal/renametype); when that lands, the
// implementation behind this interface swaps without touching
// subcommands.
type cliAnalyze interface {
    cliRead
    AnalyzeAll(ctx context.Context, opts workspace.AnalyzeOptions) *workspace.AnalysisSummary
    CheckCardinality(opts workspace.AnalyzeOptions) []workspace.CardinalityViolation
    FindDuplicates(opts workspace.AnalyzeOptions) []workspace.DuplicateGroup
    FindGaps(opts workspace.AnalyzeOptions) []workspace.GapResult
    FindOrphansWithScope(opts workspace.AnalyzeOptions) []*entity.Entity
    FindOrphanedTempFiles() ([]string, error)
    CleanupOrphanedTempFiles() (int, error)
    RunValidations(ctx context.Context, opts workspace.AnalyzeOptions) workspace.ValidationResult
    RunValidationsFiltered(ctx context.Context, opts workspace.AnalyzeOptions, filters []workspace.ValidationFilter) workspace.ValidationResult
    RenameEntityType(oldType, newType, newPlural string) (int, error)
    AttachFile(entityID, filePath, property string) (*workspace.AttachResult, error)
    ListAttachments(entityID string) ([]workspace.AttachmentInfo, error)
}
```

**`cliAnalyze` returns `workspace.*` types** (AnalysisSummary,
CardinalityViolation, etc.). That's the transitional shape — TKT-2W0X moves both
methods AND types to the lifted packages, at which point the interface
signatures change. Subcommands today already use these types; this PR doesn't
change that.

### Step 2 — Implementation struct

```go
type cliServices struct {
    ws *workspace.Workspace
}

func (s *cliServices) Store() store.Store              { return s.ws.Store() }
func (s *cliServices) Meta() *metamodel.Metamodel      { return s.ws.Meta() }
// ... thin forwarders for every method ...
```

One struct satisfies all three interfaces because workspace already provides all
the methods. The bundles are just *contracts* exposing different slices —
consumers see only what they're given via `FromContext`.

### Step 3 — Cobra context plumbing

```go
type ctxKey int

const (
    keyRead ctxKey = iota
    keyWrite
    keyAnalyze
)

func cliReadFromContext(ctx context.Context) cliRead {
    v, _ := ctx.Value(keyRead).(cliRead)
    return v
}
// ... same for Write/Analyze ...

func attachServices(ctx context.Context, svc *cliServices) context.Context {
    ctx = context.WithValue(ctx, keyRead, cliRead(svc))
    ctx = context.WithValue(ctx, keyWrite, cliWrite(svc))
    ctx = context.WithValue(ctx, keyAnalyze, cliAnalyze(svc))
    return ctx
}
```

### Step 4 — PersistentPreRunE

```go
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    configureLogging()
    if cmd.Annotations[skipProjectDiscovery] == "true" {
        out = output.New(output.Format(outputFormat))
        return nil
    }
    startDir := projectPath
    if startDir == "" {
        startDir = os.Getenv("RELA_PROJECT")
    }
    svc, err := newCLIServices(startDir)
    if err != nil {
        return wrapDiscoverError(err)
    }
    cmd.SetContext(attachServices(cmd.Context(), svc))
    out = output.New(output.Format(outputFormat))
    return nil
},
```

### Step 5 — Subcommand migration

Each subcommand pulls its bundle from the context:

```go
// before
RunE: func(cmd *cobra.Command, args []string) error {
    e, err := ws.EntityManager().CreateEntity(cmd.Context(), ...)
    ...
},

// after
RunE: func(cmd *cobra.Command, args []string) error {
    svc := cliWriteFromContext(cmd.Context())
    e, err := svc.EntityManager().CreateEntity(cmd.Context(), ...)
    ...
},
```

Each subcommand declares its bundle at the top of `RunE`. Per-file diff: ~3
lines of boilerplate + N existing `ws.X()` → `svc.X()` replacements. Mechanical.

### Step 6 — `ResolveEntityType` as a free function

Today: `ws.ResolveEntityType(typeName)` is a method on Workspace that only reads
the metamodel. Lift to a package-local free function:

```go
// internal/cli/resolveentitytype.go
func resolveEntityType(meta *metamodel.Metamodel, typeName string) (string, *metamodel.EntityDef, error) {
    // body lifted from workspace.go:666
}
```

Subcommand call sites change from `resolveEntityType(typeName)` to
`resolveEntityType(svc.Meta(), typeName)`. Removes one method from the `cliRead`
interface and one workspace coupling.

### Step 7 — Test fixture migration

`cli/test_helpers_test.go::storeSeeder.build()` returns `*cliServices` (which
satisfies all three bundle interfaces). Tests that previously did `ws =
ss.build()` now do `cmd.SetContext(attachServices(ctx, ss.build()))` — or, since
tests typically call subcommand handlers directly, use a helper
`attachServicesForTest(t, ss.build())` that returns a ready cobra command with
the context set.

**Tests stay sequential** (no `t.Parallel()` yet). The migration removes the
*blocker* — package globals — but actually adding `t.Parallel()` to ~80 tests is
its own audit job. Defer to a follow-up cleanup ticket / nit-PR.

### Alternatives considered

- **(B) Single `cliServices` bundle + cobra context.** Closes C2 but not C1. Reviewer would flag the locator-shaped struct again.
- **(C) Per-subcommand consumer interfaces (declare what each subcommand needs).** Strict CLAUDE.md reading. Rejected as scope creep — 30 interfaces, mostly with overlapping methods. Mid-grain bundles capture the real shape (read/write/analyze) without exploding surface area.
- **(D) Keep `svc` as a package global, defer cobra-context to followup.** Rejected per cranky #C2: "renaming a global is not a refactor." If we're touching every subcommand anyway, do it right.
- **(E) Pass `*cliServices` as a function parameter to each `RunE`.** Cleaner than context but requires wrapping every `RunE` registration. Context is the standard cobra-idiomatic pattern.

**Files to modify:**

- `internal/cli/cli_wiring.go` — NEW (~150 LOC: 3 interfaces, impl struct with ~25 forwarders, context plumbing, `newCLIServices` constructor).
- `internal/cli/cli_wiring_test.go` — NEW (~80 LOC: construction happy-path, context attach/extract, bundle-interface compile-time pins).
- `internal/cli/root.go` — drop `ws`/`projectCtx`/`meta` globals; update `PersistentPreRunE` to call `newCLIServices` + `attachServices`; `resolveEntityType` no longer references workspace.
- `internal/cli/resolveentitytype.go` — NEW (~30 LOC, free function lifted from workspace.go's method body).
- ~25 CLI subcommand files — add `svc := cliXFromContext(cmd.Context())` at the top of each `RunE`; replace `ws.Y()` calls with `svc.Y()`.
- `internal/cli/test_helpers_test.go` — `storeSeeder.build()` returns `*cliServices`; add `attachServicesForTest` helper.
- `internal/cli/*_test.go` — update to use context-attached services.
- `.go-arch-lint.yml` — no change (cli already imports workspace).

**Files NOT modified:**

- `internal/cli/flow.go`, `internal/cli/scheduler.go` — own one-off workspaces, separate from globals.
- `internal/cli/migrate.go`, `internal/cli/validate.go` — use free functions `workspace.Migrate()`/`workspace.Validate()`. Keep workspace import for now.
- `internal/workspace/*` — no changes. TKT-2W0X lifts facade methods later.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

Pure refactor; no new input surfaces, no new file/network access. CLI behavior
byte-for-byte preserved.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

1. **AC1 (globals gone)** — grep on `internal/cli/root.go`.
2. **AC2 (bundle interfaces with compile-time assertion)** — `cli_wiring_test.go` covers `var _ cliRead = (*cliServices)(nil)` etc.
3. **AC3 (subcommands read from context)** — grep for `FromContext(` in CLI files.
4. **AC4 (test fixture migrated)** — `cli/test_helpers_test.go::build` returns `*cliServices`. All `_test.go` files compile.
5. **AC5 (CLI race tests)** — `go test -race ./internal/cli/...` passes.
6. **AC6 (CI green)** — `just ci`.
7. **AC7 (subcommand workspace import scrubbed)** — `grep -l 'internal/workspace' internal/cli/*.go | grep -v cli_wiring\|mcp\|flow\|scheduler\|migrate\|validate\|_test` → empty.

**Edge cases:**

- **`skipProjectDiscovery` commands.** `PersistentPreRunE` short-circuits before `newCLIServices`; context has nil values. Subcommands annotated with skip don't call `cliXFromContext` (or they tolerate nil — verify during implementation).
- **Test commands that bypass `PersistentPreRunE`.** Tests use `attachServicesForTest` directly.
- **Concurrent subcommand invocation.** Cobra runs one command per process; no concurrency at the command level. Bundles are immutable after construction.

**Negative tests:**

- **`cliReadFromContext` with no services attached** returns nil. Subcommand crashes on `svc.Store()` — same loud failure as today's `ws.Store()` with nil ws. No new behavior.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Volume of mechanical edits.** ~25 CLI files. Each file: add `svc := ...FromContext(cmd.Context())` line + `s/ws./svc./`. Mitigation: scripted pass, then per-file audit. The bundle choice for each subcommand is the only judgment call — that's a small table (Step 5).
2. **`cliAnalyze` returns workspace.* types.** Transitional. Documented with `// Deprecated: TKT-2W0X` markers and a comment on the interface explaining the swap target.
3. **Test fixture migration.** `cli/test_helpers_test.go` is small (~80 LOC); the change is contained. Mitigation: do this first, verify tests compile, then migrate subcommands.
4. **Hidden direct workspace usages in subcommands.** Subcommands might reach for `workspace.AnalyzeOptions` etc. (parameter types). Mitigation: those types are exposed via `cliAnalyze`'s method signatures, so callers don't need their own `import workspace` line. Grep audit confirms.
5. **Workspace state equivalence (M1 from design review).** Confirmed: `workspace.New` does the same `bleveindex.NewMem() + factory.AddObserver` dance as `mcp_wiring.go`. The shared workspace in `cliServices` is the canonical source for all bundle methods.
6. **Cobra-context idiom.** Tests using `cmd.Execute()` flow naturally — `PersistentPreRunE` sets context. Tests calling subcommand `RunE` directly need to set context first; `attachServicesForTest` helper covers that case.

**Effort: M.** ~250 LOC new (wiring + tests + free function), ~25 files with
~3-line context-extract addition + mechanical `s/ws./svc./`. Single PR.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

N/A — internal refactor. CLI behavior unchanged. Worth documenting the
bundle/context pattern in a comment block in `cli_wiring.go` since it's a new
(within rela) pattern; CLAUDE.md cites `mcp.Services` as an example, not yet
cobra-context.

## Design Review

- [x] Run `/design-review` before starting implementation — done; round 1 results documented below
- [x] All critical/significant findings addressed in plan

**Design Review Findings (round 1):**

- **C1 (locator shape)** — addressed by splitting into 3 mid-grain bundles (cliRead/cliWrite/cliAnalyze).
- **C2 (svc as global rename)** — addressed by cobra-context passing; no package-level services global.
- **S1 (under-counted LOC)** — plan revised: ~250 LOC, not 50.
- **S2 (t.Parallel blocker)** — documented: this PR removes the blocker, t.Parallel migration is a follow-up nit.
- **S3 (validate.go shape)** — kept as transitional (free functions); explicit "files NOT modified" entry.
- **S4 (ResolveEntityType as free function)** — adopted; Step 6.
- **M1 (state equivalence)** — verified during research; Risk #5.
- **M2 (Facade naming)** — adopted; types renamed without "Facade" suffix.
- **M3 (AC2 sketched)** — replaced with explicit "compile-time assertion in cli_wiring_test.go".
- **L1 (per-subcommand interfaces incrementally)** — documented as the post-TKT-2W0X arc; not blocking this PR.
- **L2 (cobra-context exit)** — adopted as the destination, not a follow-up.

**Cranky's "verdict": Approve with C1+C2 addressed.** Both addressed in-PR. No
followup-to-followup tickets stacking.
