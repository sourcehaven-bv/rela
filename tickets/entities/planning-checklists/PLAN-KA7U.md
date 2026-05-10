---
id: PLAN-KA7U
type: planning-checklist
title: 'Planning: Extract shared workspace bootstrap helpers (production + tests)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**Problem:** workspace construction is duplicated across 3 production entry
points and ~10 test files. Adding a new required collaborator (audit, future
principal/policy) churns ~20 call sites. Concretely (file:line):

- **Production (3):** `cmd/rela-server/main.go:65`, `cmd/rela-desktop/main.go:133`, `internal/cli/root.go:86` (plus 4 CLI subcommands that bypass `root.go` because of `skipProjectDiscovery: "true"`: `flow.go:65`, `mcp.go:57`, `validate.go:119`, `scheduler.go:64`).
- **Tests (~10):** `internal/dataentry/{e2e_test.go:53, watcher_test.go:111, test_helpers_test.go:112,182,253}`, `internal/workspace/bridge_sync_test.go:69`, `internal/mcp/{watcher_test.go:41,81, tools_test.go:68}`, `internal/cli/{export_test.go:85, rename_test.go:54, test_helpers_test.go:82-84}`. Only `cli/test_helpers_test.go`'s `storeSeeder.build()` is currently factored.

**In scope:**

- **Production helper.** All 7 production sites (3 main + 4 CLI subcommands) collapse to one shared `bootstrap.Workspace(startDir, scriptExec, opts...)`. Where it lives is decided in implementation (see Approach for the trade-off). Returns `(*workspace.Workspace, error)`. Internally does `project.Discover` + `workspace.New` (or stays with `workspace.Discover` and just centralizes the call). Post-construction wiring (`StartWatching`, `scheduler.StartBackground`, app wrap) **stays caller-specific** — those genuinely differ across the three binaries and pulling them in would over-couple.
- **Test helper for shared FS-backed test setup.** Single `workspacetest.WithFS(t, meta, opts...)` (location TBD — see Approach) that centralizes the `NewForTest(meta, WithFS(fs, ctx))` pattern used by 5–6 test sites. Tests that need a memstore (`WithTestStore`) keep using their own pattern; consolidating those is out of scope (different shape).
- **Documentation update on `workspace.NewForTest`'s defaulting policy.** When a future required collaborator is added, the rule for `NewForTest` is: *auto-default to a sentinel/Nop if no option supplied; document the carve-out in the package doc comment.* This locks in the helper-first discipline so AC10-style strict rules in production don't ripple into every test.

**Out of scope:**

- The audit log itself (TKT-6YYM, blocked by this).
- Any other new collaborator on `workspace` or `WriteDeps`.
- Consolidating the `WithTestStore` pattern (different shape — those tests don't have an fs/paths to share).
- Reorganizing post-construction wiring (`StartWatching`, scheduler boot) across the 3 binaries — they genuinely differ, and unifying would over-couple.

**Acceptance Criteria:**

1. **AC1: production helper covers all 7 production sites.** `cmd/rela-server`, `cmd/rela-desktop`, `internal/cli/root.go`, and the 4 `internal/cli/*.go` files that currently call `workspace.Discover` directly all route through one shared bootstrap function. Verify by grepping for `workspace.Discover(` and `workspace.New(` outside the bootstrap package — production matches should be zero (test files exempt).
2. **AC2: validate.go's NopScriptExecutor case is preserved.** `internal/cli/validate.go:119` currently passes `workspace.NopScriptExecutor` (validation doesn't run scripts). The bootstrap helper's signature accepts `scriptExec` as a parameter so this case is preserved naturally. Test: `go build ./... && go test ./internal/cli -run TestValidate`.
3. **AC3: shared test helper covers FS-backed test sites.** The 5–6 sites using `NewForTest(meta, WithFS(...))` route through one helper. Tests still pass under `-race`.
4. **AC4: no behavior change.** Full `just ci` pipeline green: `just test`, `just lint`, `just arch-lint`, `just coverage-check`. No new "feature" tests; existing tests still pass.
5. **AC5: arch-lint clean for new package(s).** If `cmd/internal/bootstrap/` is added, `.go-arch-lint.yml` is updated and `just arch-lint` passes.
6. **AC6: `NewForTest` doc comment documents the defaulting policy.** Future-collaborator threading rule is written down in the package, not just in this plan.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing patterns:**

- `internal/cli/test_helpers_test.go:82-84` — `storeSeeder.build()` already factors the memstore-test pattern. Refactor leaves this intact; doesn't try to merge it with the FS-test pattern (different shape — memstore tests don't seed an fs).
- `workspace.Discover()` at `internal/workspace/workspace.go:145` — already wraps `project.Discover + workspace.New`, which is exactly what production sites need. The refactor mostly *centralizes* one call to it, plus adds a place to thread future collaborators.
- `workspace.NewForTest(meta, opts...)` at `internal/workspace/workspace.go:246` — variadic-options pattern, already accepts `WithFS`, `WithTestStore`, `WithScript`. The shared FS-test helper is just a thin convenience over this.

**Reference implementations:** None applicable — this is a small internal
refactor, not a problem domain anyone else has solved.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical approach:**

Two helpers. They're independent and can land in two commits.

### 1. Production helper

**Where to put it** — three options with trade-offs:

- **(A) `cmd/internal/bootstrap/`** — new package under `cmd/`. All three binaries import it. Requires adding the package to `.go-arch-lint.yml` and giving each cmd component a `mayDependOn` entry.
- **(B) `internal/cli/bootstrap.go` (or extend root.go)** — but `cmd/rela-server` and `cmd/rela-desktop` would have to import `internal/cli`, which currently they don't. Would create a weird dependency: server/desktop binaries depending on the CLI package. Reject.
- **(C) `internal/workspace/bootstrap.go`** — adjacent to the `workspace` package. Server, desktop, CLI all already import `workspace`. No new arch-lint component. The helper is a thin wrapper around `workspace.Discover`; living in the same package is honest about what it is.

**Recommendation: (C).** Smallest blast radius, no arch-lint changes, most
honest about what the helper is. Signature:

```go
// internal/workspace/bootstrap.go
package workspace

// Bootstrap is the canonical production entry point that combines project
// discovery with workspace construction. All three binaries (rela, rela-server,
// rela-desktop) and CLI subcommands route through it. Future required
// collaborators on Workspace are threaded here exactly once.
//
// startDir empty → cwd. scriptExec nil → NopScriptExecutor (per existing New
// behaviour). opts are forwarded to workspace.New.
func Bootstrap(startDir string, scriptExec ScriptExecutor, opts ...Option) (*Workspace, error) {
    fs := storage.NewSafeFS(storage.NewOsFS())
    paths, err := project.Discover(startDir, fs)
    if err != nil {
        return nil, err
    }
    return New(fs, paths, scriptExec, opts...)
}
```

This is essentially what `workspace.Discover` already does. Difference:
`Discover` returns `(*Workspace, error)` and is the historical name; `Bootstrap`
is documented as "the place where future required collaborators are threaded."
We can either:
- (i) **Keep both** — `Discover` is the low-level path-discover-and-construct, `Bootstrap` is the policy-bearing entry point that future collaborator wiring adds to.
- (ii) **Just use `Discover` directly** and document that as the canonical entry point, no new function.

(ii) is simpler. The "place where future collaborators thread" doesn't need a
new function name — it can be `Discover`. Naming a function `Bootstrap` to mean
"future people will thread things here" is a comment, not a contract.

**Final recommendation:** **No new production function.** Keep
`workspace.Discover`. The actual refactor work is:
- Update `cmd/rela-server/main.go:65`, `cmd/rela-desktop/main.go:133`, `internal/cli/root.go:86` to all use `Discover` (rela-desktop currently uses `New` after manual `discoverProject()`; flatten that to `Discover`).
- Update the 4 CLI subcommands (`flow.go`, `mcp.go`, `validate.go`, `scheduler.go`) — they already call `Discover`, no change needed; they're already centralized at the *function* level even though each call site is its own line.
- Add a doc comment on `workspace.Discover` explicitly stating: *"This is the canonical production entry point. Future required collaborators are threaded here, then through `New`. Adding a new required collaborator means: (1) add Option helper in workspace; (2) thread it from Discover into New; (3) callers pass via `opts...` if they need a non-default value."*

The actual code change for production is small — `cmd/rela-desktop/main.go`
flattens `discoverProject + workspace.New` into `workspace.Discover`. The doc
comment is the load-bearing artifact.

### 2. Test helper

**Where:** `internal/workspace/workspacetest/` (new sub-package). Out of
`_test.go` so multiple packages can import it. Naming follows the existing
`internal/store/storetest` precedent.

**Signature:**

```go
// internal/workspace/workspacetest/workspacetest.go
package workspacetest

func WithFS(t testing.TB, meta *metamodel.Metamodel, opts ...workspace.TestOption) *workspace.Workspace {
    t.Helper()
    fs, paths := newProjectFS(t)  // creates a tmp project tree
    allOpts := append([]workspace.TestOption{workspace.WithFS(fs, paths)}, opts...)
    return workspace.NewForTest(meta, allOpts...)
}
```

`newProjectFS(t)` extracts the boilerplate currently inlined in each dataentry
test helper. Specifics (what dirs to create, what defaults) come from auditing
those 5–6 sites; if they diverge meaningfully, the helper accepts an option for
that diverging concern rather than smearing it across callers.

### 3. `NewForTest` defaulting policy

Add a doc comment to `workspace.NewForTest`
(`internal/workspace/workspace.go:246`):

> When a new required collaborator is added to `workspace.New` (e.g. via a `WithX(x)` option that production must always pass), `NewForTest` should *auto-default* to a sentinel/Nop implementation if the corresponding `WithX` option is not supplied. Production `New` may reject nil; `NewForTest` is allowed to pre-populate. This keeps test-site churn bounded when collaborators are added.

This is the actual policy artifact. Without it, the next required-collaborator
addition fights the same battle.

**Files to modify:**

- `internal/workspace/workspace.go` — extend `Discover` doc comment; extend `NewForTest` doc comment with defaulting policy.
- `cmd/rela-desktop/main.go` — flatten manual `discoverProject + workspace.New` to `workspace.Discover` (if and only if `discoverProject` returns nothing extra that's needed; verify during implementation — the current code returns `(fs, *project.Context, error)`, so check whether `fs` is used elsewhere after construction).
- **New:** `internal/workspace/workspacetest/workspacetest.go` + small smoke test.
- `internal/dataentry/test_helpers_test.go` — three call sites (lines 112, 182, 253) migrate to `workspacetest.WithFS`.
- `internal/dataentry/watcher_test.go:111` — same migration.
- `internal/cli/rename_test.go:54` — same migration.
- `internal/mcp/watcher_test.go:41,81` — same migration if they fit (real FS, no `WithTestStore`).

**Files explicitly NOT modified:**

- `internal/cli/test_helpers_test.go` — `storeSeeder.build()` is the right shape for memstore-seeded tests; leaving alone.
- `internal/cli/export_test.go:85`, `internal/mcp/tools_test.go:68` — both use `WithTestStore`, different shape; leaving alone.
- `cmd/rela-server/main.go:65` — already uses `Discover` directly. No change.

**Alternatives considered (rejected):**

- **`cmd/internal/bootstrap/` package** — overkill for what amounts to "use `Discover` consistently and document it." Adds an arch-lint touchpoint for no real benefit.
- **A new `Bootstrap()` function alongside `Discover()`** — naming a function to mean "future people will thread things here" is a comment, not a contract. Just document `Discover` and use it.
- **Consolidating `WithTestStore` and `WithFS` test patterns into one helper** — different shape (memstore tests don't seed an fs), forced unification would obscure the distinction.
- **Moving post-construction wiring (StartWatching, scheduler) into the bootstrap helper** — the three binaries genuinely differ here (server starts scheduler unconditionally; desktop starts per-project with cancellable ctx; CLI doesn't start it at all). Pulling these in would over-couple.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:** None — this is a refactor that moves code, not
new input handling. `startDir` is already validated by `project.Discover`.

**Security-sensitive operations:** None new. Existing path validation in
`storage.SafeFS` and `project.Discover` is unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Integration test approach defined

**Test scenarios:**

- AC1 / AC2: Verified by `go build ./...` (compilation) + `just test` (existing tests). No new tests required for "production sites use `Discover`" — the existing test suites exercise the production paths.
- AC3: Existing tests under `internal/dataentry`, `internal/workspace`, `internal/mcp`, `internal/cli` pass after migration.
- AC4: `just ci` green.
- AC5: `just arch-lint` green (only triggers if we add a new package; current recommendation is no new package).
- AC6: Smoke test in `internal/workspace/workspacetest/workspacetest_test.go` exercising `WithFS` + assertion on returned workspace.

**Edge cases:**

- `cmd/rela-desktop`'s `discoverProject` returns `fs` separately — verify it's used after construction (if so, the flatten doesn't apply and we keep the explicit `New` call).
- Tests using `WithFS` with custom `WithStoreFactory` or `WithScript` — make sure the `workspacetest.WithFS` helper accepts arbitrary additional `TestOption`s so callers aren't constrained.
- A test that builds a workspace with a *partial* fs setup (no metamodel, etc.) — current `NewForTest` allows this; the helper must too.

## Risk Assessment

- [x] Technical risks assessed
- [x] Effort estimated

**Risks:**

| Risk | Mitigation |
|---|---|
| `cmd/rela-desktop`'s `discoverProject` may use the `fs` it returns after construction (e.g. for a separate file read), preventing a clean flatten | Verify during implementation. If true, leave the explicit `New` call and document why. The doc-comment-on-Discover artifact is still valuable. |
| `workspacetest` helper is too generous and masks real bugs | Keep helper narrow: just centralize the `WithFS(fs, paths)` part; do not auto-create test fixtures or seed data. |
| The "doc comment as policy" is weak — future authors won't read it | Mitigated only partially. Consider adding a CLAUDE.md note pointing at the policy. Real teeth would require an arch-lint rule or a linter, both out of scope. |
| Refactor accidentally changes behavior (e.g. different default for omitted Option) | `just ci` passing (which includes `-race`) is the gate. Manually exercise `rela-server` and `rela-desktop` startup once locally. |

**Effort: s.** Roughly: ½ day audit of call sites + flatten desktop path; ½ day
extracting `workspacetest`; ½ day migrating test sites + verifying `just ci`; ½
day docs + cleanup. ~2 days end-to-end.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [ ] User guide / reference docs — N/A (internal refactor).
- [ ] CLI help text — N/A (no commands changed).
- [x] CLAUDE.md — add a one-line entry under "Rules for new code" pointing at the `workspace.Discover` doc comment as the canonical place to thread new required collaborators in production, and at the `NewForTest` doc comment for the test-side defaulting policy.
- [ ] README.md — N/A.
- [ ] API docs — N/A.

## Design Review

- [ ] Plan reviewed (`/crit`, cranky-code-reviewer, go-architect)
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** *to be populated after review*
