---
id: PLAN-KAK2R
type: planning-checklist
title: 'Planning: Surface Lua errors from validation rules'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Validation rules in `internal/validation/lua.go` execute Lua via raw `ls.PCall`
and swallow compile/runtime errors via `slog.Warn` (fail-open). Operators
running `rela analyze` (or `rela validate`) get no diagnostic about why a rule
produced no violations — only its absence.

**Scope (in):**

- Wrap Lua compile + runtime errors in `validateLua` as `*lua.ScriptError`
with `Surface = "validation"`.
- Surface those errors to `rela analyze` and `rela validate` CLI output
as a typed `validation.Result` struct with three slices: `Violations`,
`ScriptErrors`, `LoadErrors`.
- Preserve fail-open semantics: a single broken rule must not abort the
entire validation pass; remaining rules and entities still run.
- Cover the inline `lua:` block path AND the `lua_file:` path.
- Source slice via `os.DirFS(s.deps.ProjectRoot)` (project root, NOT
validations/ subdir) so context lines render for `lua_file:` rules, matching the
automation surface's pattern in `script/executor.go`.
- Hoist runtime construction from per-(rule, entity) to per-rule so
module-local Lua memoization persists across entities and the `WithCache`
rationale comment becomes accurate (drive-by from design review).
- Thread `context.Context` through `Service.Check` /
`CheckRule` / `CheckRules` so operator Ctrl+C cancels long-running rules
(drive-by from design review).
- Bring "rule must return nil or table" / "missing message field"
contract errors into the envelope as `*ScriptError` (synthesized, no LuaLine) —
operator UX deserves the same visibility.

**Scope (out):**

- Changing fail-open to fail-closed (out — separate decision).
- Threading `*ScriptError` to MCP `analyze_validations` tool (follow-up
if/when MCP needs it; pure CLI improvement now).
- AI-in-validation (already deferred per TKT-LR5YC).
- `parseLuaReturnValue` misclassifying tables that are both array and
carry a `message` key — pre-existing bug, file follow-up.

**Acceptance Criteria:**

1. **AC1 — Compile error in inline `lua:` block surfaces as ScriptError.**
A rule with `lua: "if oops invalid"` runs `rela analyze`; the
`validation.Result` contains a `*ScriptError` with Surface=`"validation"`,
Path=`validations/<rule-name>`, non-empty LuaMessage. CLI output renders it
readably (path:line: message).

2. **AC2 — Runtime error in `lua_file:` rule shows source slice.** A
rule with `lua_file: foo.lua` containing `local x = nil; return x.field` runs;
the `*ScriptError` has non-empty Source (3 lines above + 3 below the failing
line) and Path=`validations/foo.lua`, resolved against `os.DirFS(projectRoot)`.

3. **AC3 — Fail-open preserved.** Five rules where one has a Lua error
and four are valid: the four still run, their `Violations` still appear; the
broken one produces a `ScriptError` entry. Service does not return early.

4. **AC4 — 5s per-rule timeout enforced.** A rule that runs
`while true do end` returns a context-deadline-exceeded `*ScriptError` within
~5s; the next rule executes normally with its own fresh timeout (verified by
total wall-clock < 7s for two such rules running back-to-back).

5. **AC5 — Unchanged "no Lua" path.** Rules with neither `lua:` nor
`lua_file:` continue to short-circuit (no envelope, no slog).

6. **AC6 — `lua_file: foo.lua` not found surfaces as LoadError.** A
rule referencing a missing file produces a `Result.LoadErrors[0]` entry with
rule name + sanitized message, distinct from `ScriptErrors`.

7. **AC7 — Contract violations surface as ScriptError.** A rule
returning a number (not nil/table) produces a synthesized `*ScriptError` with
LuaMessage = "rule must return nil or table, got number"; rule with violation
table missing `message` key produces analogous error.

8. **AC8 — Context cancellation propagates.** Cancelling the parent
context passed to `Service.Check` interrupts an in-flight Lua rule with a
context.Canceled error wrapped in `*ScriptError`.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **TKT-LR5YC merged in develop**: `*lua.ScriptError`, `BuildScriptError`,
PCall message handler. Five surfaces wired there are the reference
implementation. Files: `internal/lua/scripterror.go`,
`internal/lua/runtime.go:331–375`.
- **`internal/script/executor.go:139-149` `wrapScriptError`**: the
canonical pattern for the envelope-path/SourceFS/frame-rewrite contract.
Validation should match this pattern; cleaner because validation can pass the
envelope path as chunkname and skip frame rewriting entirely.
- **Runtime API**: `RunString` / `RunFileContent` discard return
values. `RunActionString` returns one value via `luaValueToGo` (loses
`*lua.LTable`). Need a new method.
- **Reader runtime is constructed via `lua.NewReader(deps,
io.Discard, opts...)`** — accepts `lua.WithTimeout` and `lua.WithContext` opts.
Plan uses both.

**Files reviewed:**

- `internal/validation/lua.go` (260 LOC)
- `internal/validation/validation.go` (Service, Check/CheckRule/
CheckRules/runLuaValidation)
- `internal/validator/validator.go` (downstream consumer of
`Service.Check` — `Result` ripple)
- `internal/cli/analyze.go:398`, `internal/cli/validate.go:283/285`
(CLI consumers via `RunValidations` / `RunValidationsFiltered`)
- `internal/lua/runtime.go` (Runtime API, PCall handler,
`applyTimeout`)
- `internal/lua/scripterror.go` (BuildInput, redaction)
- `internal/script/executor.go` (envelope-path pattern reference)
- `internal/workspace/workspace.go` and `workspace/analysis.go`
(RunValidations wrapper, Service factory + cache wiring)

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Five sub-decisions; each lands as a discrete commit so review is easy.

### Decision 1 — How to capture frames from a Lua call that returns a value

**Chosen:** Add `Runtime.RunValidationString(code, chunkname string)
(golua.LValue, error)`. Returns the script's first return value as a raw
`golua.LValue`. Internally calls `r.L.Load(strings.NewReader(code), chunkname)`
(so chunkname becomes the frame path), then `r.L.Push(fn)` +
`r.pcallWithCapture()`. Reads top-of-stack and returns it untouched.

`applyTimeout()` is invoked before `Load` (matching other Run* methods) so
`WithTimeout` / `WithContext` opts on the Runtime are honored.

Rationale:

- Validation needs `*golua.LTable` access; `RunActionString` strips
that via `luaValueToGo`.
- New method, not modification of existing — keeps existing API stable.

Alternative rejected: copy `pcallWithCapture` into validation (duplicates the
message-handler code, drifts as runtime evolves).

Alternative rejected: export `collectStackFrames` and let validation build its
own frames (validation would still re-implement the message handler).

### Decision 2 — How errors travel from Service to CLI — RESOLVED (Shape A)

**Chosen:** Shape A with a named result struct, three slices.

```go
// in package validation
type Result struct {
    Violations   []Violation
    ScriptErrors []*lua.ScriptError
    LoadErrors   []LoadError
}

type LoadError struct {
    RuleName string
    Message  string // already sanitized by loadLuaScript
}

func (r Result) HasErrors() bool {
    return len(r.ScriptErrors) > 0 || len(r.LoadErrors) > 0
}

func (s *Service) Check(ctx context.Context, entities []*entity.Entity, scope map[string]bool) Result
func (s *Service) CheckRule(ctx context.Context, rule metamodel.ValidationRule, entities []*entity.Entity, scope map[string]bool) Result
func (s *Service) CheckRules(ctx context.Context, entities []*entity.Entity, scope, ruleNames map[string]bool) Result
```

Three slices because the three classes of failure are semantically distinct (per
design review #5):

- `Violations`: rule ran successfully and the entity violated it.
- `ScriptErrors`: rule loaded but Lua failed (compile, runtime,
timeout, or contract violation like wrong return shape).
- `LoadErrors`: rule's `lua_file:` couldn't be opened (config-level
failure, no Lua VM ever ran).

CLI rendering treats them as three distinct sections.

Alternatives considered and rejected (architect verdict + design review):

- **B (synthesize Violation from ScriptError)**: category error per
go-architect — `Violation` means "entity violated rule"; a `*ScriptError` means
"rule did not run." Conflating produces lies (EntityID half-truth,
RuleName-grouped filters mix failures with findings) AND throws away the typed
envelope.
- **C (additive accessor)**: mutable accumulator on long-lived
Service — concurrency + lifecycle questions with no good answer.
- **A but with two slices** (no separate LoadErrors): violates
honest categorization — load failure has no Lua frames, no LuaLine, no source
slice; shoving it into `*ScriptError` makes the envelope dishonest and confuses
downstream renderers.

### Decision 3 — Path identity, SourceFS, and chunkname matching

**Chosen:** Match `script/executor.go:wrapScriptError` pattern, with the
simplification that we control the chunkname.

For inline `lua:` blocks:

- Envelope `Path` = `"validations/" + rule.Name`
- Chunkname passed to `RunValidationString` = same string
- `SourceFS` = nil (no on-disk file; no source slice)

For `lua_file:` rules:

- Envelope `Path` = `"validations/" + filepath.ToSlash(scriptPath)`
(e.g., `validations/foo.lua`)
- Chunkname passed to `RunValidationString` = same string
- `SourceFS` = `os.DirFS(s.deps.ProjectRoot)` — project root, NOT
the validations subdir. `readSourceSlice` looks up `validations/foo.lua` from
project root.

Because chunkname == envelope path, frames captured by `pcallWithCapture`
already have `Path == envelopePath`. No frame-rewriting needed (the
simplification mentioned above).

This matches TKT-LR5YC's `automation:<name>` convention shape (typed surface
prefix + identifier).

### Decision 4 — Runtime lifecycle: per-rule, not per-(rule, entity)

**Chosen:** Construct the Lua runtime once per rule (outside the entity loop)
and re-apply `entity` global per entity within that rule.

Today: `validateLua(entity, rule)` is called per entity per rule from inside
`runLuaValidation` (called from `CheckRule` per entity). Each call constructs a
fresh `lua.NewReader(...)` and closes it.

After: `runLuaValidation` becomes the runtime's owner. It builds the runtime
once per rule, iterates entities, sets `entity` global per iteration, calls
`RunValidationString(code, envelopePath)`, collects violations or errors. On
`defer`, closes the runtime.

Effect:

- Module-local Lua memoization (`local cache = {}` at script top)
works across entities for the same rule, as the `WithCache` rationale comment
claims.
- ~N× cheaper construction (N = entity count per rule).
- No semantic regression: `entity` global is reset per iteration; no
other globals leak.

Rationale: design review #4. The existing comment at `lua.go:92-101` implied
"memoize expensive lookups across entities" — that's only true via
`*lua.Cache`'s process-shared backing today. After this change, the comment
becomes truthful for both `*lua.Cache` and module-local Lua memoization.

### Decision 5 — Timeout and context plumbing

**Chosen:** Construct the per-rule runtime with both
`lua.WithTimeout(validationTimeout)` and `lua.WithContext(ctx)`, where `ctx` is
the parent context threaded through `Service.Check(ctx, ...)`.

Effect:

- `Runtime.applyTimeout()` (called inside `RunValidationString`
before `r.L.Load`) derives a 5s timeout rooted at the parent context. Cancelling
`ctx` cancels in-flight Lua. Hitting 5s raises `context.DeadlineExceeded` from
the gopher-lua VM.
- `BuildScriptError`'s existing fallback path
(`TestBuildScriptError_PlainGoError`) handles non-ApiError errors; context
errors flow through unchanged with `Surface=validation`,
`LuaMessage=err.Error()`, no LuaLine.

Drops the manual `ls.SetContext(...)` from current code — the runtime owns
timeout/cancellation now, consistent with every other surface.

`CheckRule` and `validateLua` accept the ctx parameter; CLI commands pass
`cmd.Context()` (cobra-rooted, Ctrl+C aware). Tests pass `context.Background()`
or a manual timeout for AC4/AC8.

### Decision 6 — Synthesized contract-violation errors

**Chosen:** Replace `slog.Warn` for "must return nil or table" and "missing
message field" cases with synthesized `*ScriptError`:

```go
// e.g. for return-shape violation:
se := &lua.ScriptError{
    Surface:    lua.SurfaceValidation,
    Path:       envelopePath,
    LuaMessage: fmt.Sprintf("validation rule must return nil or table, got %s", ret.Type().String()),
}
```

Same surface, same envelope shape, no LuaLine (no frames). CLI renders
consistently. Operator visibility = same as Lua errors.

`Surface` constant added: `SurfaceValidation = "validation"` in
`internal/lua/scripterror.go` (alongside the existing five).

**Files to modify:**

- `internal/lua/scripterror.go` — add `SurfaceValidation` constant.
- `internal/lua/runtime.go` — add `RunValidationString(code, chunkname)
(golua.LValue, error)` (~25 LOC).
- `internal/validation/validation.go` — define `Result` and
`LoadError`; change `Check`, `CheckRule`, `CheckRules` to `(ctx, ...) Result`;
restructure `runLuaValidation` to own the runtime per rule and accept ctx.
- `internal/validation/lua.go` — replace raw `ls.PCall` and
`ls.SetContext` with `RunValidationString`; build `*ScriptError` via
`BuildScriptError` for runtime/compile/timeout failures; synthesize
`*ScriptError` for return-shape contract failures; return `LoadError` for
`loadLuaScript` failures.
- `internal/validator/validator.go` — propagate the new
`Result` shape; render the three slices.
- `internal/workspace/workspace.go` — `RunValidations` /
`RunValidationsFiltered` propagate `validation.Result` and accept ctx. Per
CLAUDE.md workspace is transitional; widening an existing return type and adding
a ctx param is fine — not adding new methods.
- `internal/cli/analyze.go` / `internal/cli/validate.go` — render
the three slices as separate sections; pass `cmd.Context()` to
`RunValidations*`.
- `internal/dataentry/analyze.go` (if it calls `RunValidations`) —
pass `r.Context()`.
- `internal/validation/lua_test.go` — extend; cover compile error,
runtime error, timeout, fail-open with mixed rules, contract violations, load
error.
- New: `internal/validation/lua_scripterror_test.go` — focused
tests for envelope path/source/frames in validation context.
- New: `internal/validation/lua_lifecycle_test.go` — verify
per-rule runtime construction (only one runtime per rule regardless of entity
count).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Lua code** comes from `metamodel.yaml` (`lua:`) or files under
`validations/` (`lua_file:`). Source: project author. Trusted relative to the
rest of the metamodel — Lua here can already see the entity graph via the reader
runtime.
- **Path traversal on `lua_file:`**: existing `loadLuaScript` uses
`os.OpenRoot` (TOCTOU-resistant, blocks symlink escapes via Go 1.20+ semantics).
Source-slice reading uses `os.DirFS` which rejects `../` escapes from project
root but follows symlinks *within* the rooted tree. Threat model: project author
writes their own metamodel; the attacker is themselves. Acknowledged asymmetry —
source slices are best-effort cosmetic context, not a security boundary. (Design
review #6.)

**Security-Sensitive Operations:**

- **Source slice exposure**: validation rules contain logic written
by project maintainers; source slices in CLI output are appropriate. Unlike
data-entry's loopback gating, no remote consumer to gate against.
- **Sanitized error messages**: `loadLuaScript` deliberately scrubs
paths from some error messages ("cannot access project directory" etc).
`LoadError.Message` keeps the existing scrubbed text — we don't re-leak by
passing it through `BuildScriptError`. (Design review #5.)
- **Redaction**: `BuildScriptError` already redacts `Args` and
`CapturedOutput`. Validation passes neither → redaction is a no-op but harmless.
- **No new secret-bearing fields**.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| AC1 (inline compile error) | `lua_test.go`: rule with `lua: "if oops invalid"`, assert `Result.ScriptErrors[0]` has Surface=`SurfaceValidation`, Path=`validations/<rule-name>`, LuaMessage non-empty |
| AC2 (file runtime error w/ source slice) | new test in `lua_scripterror_test.go`: t.TempDir as projectRoot; write `validations/foo.lua` with broken script; construct `validation.New(meta, lua.ReadDeps{ProjectRoot: tmp})`; assert `ScriptErrors[0].Source` non-empty, `Path == "validations/foo.lua"`, source contains the failing line text |
| AC3 (fail-open) | table-driven: 5 rules (1 broken + 4 valid); assert `Result.Violations` reflects 4 valid, `Result.ScriptErrors` len==1, Service didn't return early |
| AC4 (timeout per rule) | rule A: `while true do end` (5s timeout fires), rule B: trivial; assert A produces ScriptError, B Violation present, total wall < 7s |
| AC5 (no-lua path unchanged) | existing tests pass without modification |
| AC6 (LoadError) | rule with `lua_file: missing.lua`; assert `Result.LoadErrors[0]` populated with rule name + sanitized message; `ScriptErrors` empty for that rule |
| AC7 (contract error envelope) | rule `lua: "return 42"`; assert `ScriptErrors[0].LuaMessage` mentions "rule must return nil or table". Rule `lua: "return {}"` (empty table — no message): assert `ScriptErrors[0].LuaMessage` mentions "missing message field" |
| AC8 (context cancellation) | parent ctx with cancel; rule with `while true do end`; cancel ctx after 100ms; assert ScriptError appears within ~150ms (not 5s) |

**Edge Cases:**

- **Empty `lua:` and `lua_file:`** → still short-circuits at `lua.go:76`. Re-verify in test.
- **`lua_file: ../etc/passwd`** → already rejected by `os.OpenRoot` in `loadLuaScript`. Test asserts `LoadError`, not `ScriptError`.
- **`lua_file: foo.lua` not found** → `LoadError`, not `ScriptError` (AC6).
- **Lua returns a non-table** → synthesized `ScriptError` (AC7).
- **Lua returns table missing `message`** → synthesized `ScriptError` (AC7).
- **Concurrent `Service.Check` calls (data-entry HTTP handler)** → safe: per-call construction of fresh runtime; `*lua.Cache` is shared and internally synchronized. (Design review #8.)
- **Race detector** → existing tests run with `-race`; no new global state.

**Negative Tests:**

- Lua syntax error → `*ScriptError` with LuaMessage from `*ApiError`.
- Runtime nil-index → `*ScriptError` with LuaLine from frames.
- Timeout (5s budget) → `*ScriptError`; verify `BuildScriptError` handles non-ApiError gracefully.
- Cancelled context → `*ScriptError` with `context.Canceled` flowing through.

**Integration test approach:**

- End-to-end test: t.TempDir; write metamodel.yaml with mixed valid + broken Lua rules; write `validations/broken.lua`; run `Service.Check(ctx, ...)`; assert all three slices populated correctly.
- CLI-level rendering covered by unit test on a `formatScriptError` helper. No golden file (too brittle).

**Lifecycle test (Decision 4 verification):**

- Inject a counter into `lua.NewReader` (or wrap/spy) and verify in a multi-entity multi-rule setup that runtime construction count == rule count, not rule × entity count.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) → **m** (was m before drive-bys; still m after — drive-bys are small and adjacent)

**Risks:**

- **R1 — `RunValidationString` API drift.** Adding a fourth Run*
method may attract criticism. Mitigation: keep small, document the divergence
(typed return value); architect already weighed in.
- **R2 — `validation.Result` ripple.** Five Service call sites + 2
workspace methods + multiple CLI/dataentry/validator/test sites change.
Mitigation: each as a discrete commit; tests pinned before/after; lint +
arch-lint catch unintended import widening.
- **R3 — Fail-open regression.** If `runLuaValidation` is
restructured clumsily, might short-circuit. Mitigation: AC3 covers explicitly
with multi-rule test.
- **R4 — Per-rule runtime hoisting introduces cross-entity state
leak.** If `entity` global isn't reset cleanly, rule sees stale entity.
Mitigation: explicit unit test asserting that two consecutive entities receive
their own `entity` global value; `SetGlobal` overwrites — should be safe.
- **R5 — Context plumbing breaks tests.** Tests calling
`Service.Check(entities, nil)` need `Service.Check(ctx, entities, nil)`.
Mechanical fix; ~30 call sites by grep estimate. Mitigation: do this change
first as a no-op pass (signatures threaded with `context.TODO()`), then layer in
semantic changes.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — N/A: existing validation docs
don't promise specific failure-mode behavior.
- [ ] CLI help text — N/A: no new flags or commands.
- [x] CLAUDE.md — small note in the "Lua surfaces" section if such
a section exists; otherwise N/A.
- [ ] README.md — N/A.
- [ ] API docs — N/A.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** *(folded into plan above; key changes:
WithTimeout+ctx plumbing, envelope-path/SourceFS rooted at project root with
chunkname matching, per-rule runtime hoisting, separate LoadErrors slice,
contract-violation envelope, context-cancellation AC, concurrency note,
t.TempDir test specifics)*
