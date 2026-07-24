---
id: IMPL-3N1D8Z
type: implementation-checklist
title: 'Implementation: rela-docs phase 2 (Tier A): markdown+Lua-island doc language + schema/graph resolvers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `internal/mermaid` (10 tests incl. injection), `internal/docs/preprocess_test.go` (parser), `internal/docs/build_test.go` (resolvers + pipeline)
- [x] Integration tests written — `build_test.go` drives the full Build() over fixture metamodel + policy + seeded memstore; smoke-tested `rela docs build` against the real `prototypes/data-entry/project`
- [x] Happy path implemented — all 9 resolvers + emit + seed + `rela docs build` CLI, verified end-to-end on a real project
- [x] Edge cases from planning handled — flat-enum lifecycle fallback, exclude/only conflict, empty resolve (strict vs warn), unknown type/field, diamond-graph dedupe, echo-vs-prose disambiguation, seed-any-status (raw store)
- [x] Error handling in place — fail-loud `BuildError` with the manual source line; Lua errors, resolver errors, parse errors all surface with `manual:N`

## Test Quality

- [x] Using fixture builders — `fixtureMeta(t)` / `fixturePolicy()` builders; shared `build(t, src, opts)` helper
- [x] No hardcoded values where object is in scope — assertions check emitted structure against the fixture
- [x] Only specifying values that matter — fixtures carry the minimum to exercise each resolver
- [x] Interpolated values from objects — seed round-trips via `create` then `entity{id=r.id}`
- [x] Property comparisons use original object — diamond test counts actual edges

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./internal/docs/ ./internal/mermaid/` — PASS. Coverage: **docs 81.9%**, **mermaid 96.6%** (both well above the 50% floor; no override needed).
- `just coverage-check` — PASS (package 50% + total 65%; total 76.3%).
- `just arch-lint` — OK (new `mermaid` + `docs` components + `mayDependOn`/`canUse` added).
- `golangci-lint run ./internal/docs/... ./internal/mermaid/... ./internal/cli/...` — 0 issues.
- `just lint-md` — 0 issues. `just fmt` clean.
- `go build ./...` + `go test ./...` — full suite green (73 packages ok, 0 failures).
- **End-to-end on the real prototype** (`rela docs build --project prototypes/data-entry/project prototypes/data-entry/manual/tickets-manual.md`): renders the deployment description, required-field table, a **mermaid stateDiagram** (Start work/Mark resolved/Close/Reopen), per-value meanings + labels + default, relations list, schema graph (incl. the `blocks`→ticket self-loop), a seeded worked example (create→entity), the echo count, and the editor/viewer roles matrix.
- AC-by-AC: AC1 statement `TestBuild_StatementIsland`; AC2 echo `TestBuild_EchoCount` + coercion + prose-disambiguation `TestParse_RelaProseMentionNotEcho`; AC3 `TestBuild_TyperefRequired`; AC4 `TestBuild_ValuesWithDescriptions`; AC5 `TestBuild_LifecycleDiagram`/`_FlatFallback` + mermaid injection tests; AC6 `TestBuild_GraphInstanceDiamond`/`_GraphSchema`/`_GraphExclude`/`_ExcludeOnlyConflict`; AC7 `TestBuild_RolesMatrix`/`_NoPolicy`; AC8 `TestBuild_SeedRawStoreNoGate`; AC9 `TestBuild_FailLoudUnknownType`/`_StrictEmptyResolve`; AC10 the example manual; AC11 mermaid extraction + dataentry tests green; AC12 `TestBuild_InfiniteLoopTimesOut` (0.5s deadline hit).

## Quality

- [x] Code follows project patterns — mermaid extraction mirrors the existing dataentry renderer; doc `doc.*` module registered `registerCryptoModule`-style (closures, no `*Runtime` methods → plimsoll flat); reuses `lua.NewReader`, `EntityToTable`, `tracer`, `memstore`, `acl.grantsVerb` logic
- [x] Checked for DRY — extracted `internal/mermaid` (shared with dataentry), `enumInfo`/`schemaBuilder` where it sharpened the contract; kept small helpers inline otherwise
- [x] No security issues introduced — sandbox (no io/os) inherited; mermaid injection-safe (synthetic ids + newline flatten, tested); seed writes isolated to a throwaway memstore; manual/output paths are operator-trust (plain os.ReadFile/WriteFile, `--out` refuses a directory)
- [x] No silent failures — fail-loud `BuildError` (returned, not logged-and-dropped); empty resolve warns (non-strict) or errors (strict)
- [x] No debug code left behind
