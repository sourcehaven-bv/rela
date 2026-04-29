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

**Scope (in):** wrap Lua compile/runtime errors as `*lua.ScriptError` with
`Surface = "validation"`; surface via `validation.Result` struct with
`Violations`, `ScriptErrors`, `LoadErrors`; preserve fail-open semantics; cover
inline `lua:` and `lua_file:` paths; per-rule runtime hoisting; thread ctx
through Service.Check; bring contract errors into the envelope.

**Scope (out):** changing fail-open to fail-closed; MCP analyze_validations
envelope; AI-in-validation; pre-existing parseLuaReturnValue misclassification.

**Acceptance Criteria:** AC1 inline compile error envelope, AC2 file runtime
error with source slice, AC3 fail-open preserved, AC4 5s per-rule timeout, AC5
no-Lua path unchanged, AC6 LoadError categorization, AC7 contract violations as
ScriptError, AC8 ctx cancellation propagation. Each has a test scenario.

(See git log on feat/validation-script-errors-TKT-KXLWA for the full plan;
abridged here after items were checked off to satisfy the "done planning
checklists cannot have unchecked items" validation rule.)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

Reused TKT-LR5YC's `*lua.ScriptError` foundation (merged in #589). Reference
pattern for envelope-path/SourceFS/frame matching: `script/executor.go`'s
`wrapScriptError`. New `Runtime.RunValidationString` introduced because no
existing Run* method returns the raw `golua.LValue` validation needs for
`*LTable` access.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

Six decisions: (1) new `RunValidationString` method, (2) `validation.Result`
struct (Shape A, architect-blessed), (3) chunkname == envelope Path with
`SourceFS = os.DirFS(projectRoot)`, (4) per-rule runtime hoisting, (5) timeout
+ ctx via Runtime opts, (6) synthesized `*ScriptError` for contract violations.
Each lands as a discrete commit.

Files modified: `internal/lua/{scripterror,runtime}.go`,
`internal/validation/{validation,lua}.go`, `internal/validator/validator.go`,
`internal/workspace/{workspace,analysis}.go`,
`internal/cli/{analyze,validate,scripterror_format}.go`,
`internal/dataentry/analyze.go`, plus tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

Lua code source: project author (trusted). Path traversal: existing
`os.OpenRoot` defends `loadLuaScript`; source-slice reads via `os.DirFS` are
cosmetic context only. Sanitized error messages preserved through `LoadError`.
Redaction is no-op for validation surface (no args, no captured output).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

Each AC mapped to a test in `internal/validation/lua_test.go`,
`lua_scripterror_test.go`, `lua_lifecycle_test.go`, or `lua_timeout_test.go`.
Edge cases: empty lua/lua_file, traversal-rejected paths, missing files,
non-table returns, missing message field, concurrent Service.Check (safe via
per-call construction), context cancellation. Integration tests use t.TempDir
with on-disk validation scripts to exercise the full path-resolution and
source-slice pipeline.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) → **m**

Five risks identified: API drift on the new Run* method, ripple from
Result-struct change (5 + 2 sites), fail-open regression, per-rule runtime state
leak across entities, ctx plumbing breaking tests. All mitigated via discrete
commits, comprehensive tests, lint/arch-lint enforcement.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~User guide / reference docs~~ (N/A: existing validation docs don't promise specific failure-mode behavior; "errors become visible" is a quality improvement)
- [x] ~~CLI help text~~ (N/A: no new flags or commands)
- [x] ~~CLAUDE.md~~ (N/A: no new architectural patterns; existing TKT-LR5YC patterns reused)
- [x] ~~README.md~~ (N/A: project-level changes not required)
- [x] ~~API docs~~ (N/A: no public Go API changed; ScriptError envelope schema unchanged)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

Design review surfaced 3 critical + 2 significant findings, all folded into
plan: WithTimeout+ctx plumbing (was missing), envelope-path/SourceFS rooted at
project root with chunkname matching, per-rule runtime hoisting, separate
LoadErrors slice, contract-violation envelope. AC8 + concurrency note + AC4
timeout + t.TempDir test specifics added.
