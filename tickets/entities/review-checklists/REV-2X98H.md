---
id: REV-2X98H
type: review-checklist
title: 'Review: Replace lua.Services struct with minimal consumer interfaces per call site'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — all 39 packages pass; coverage unchanged or improved
- [x] ~~Lint clean (`just lint`)~~ (N/A: pre-existing golangci-lint config bug with `output.formats` format, unrelated to this branch; verified by running lint against `develop`)
- [x] Coverage maintained — scheduler 71.5%→77.1%, lua 84.4%→84.4%, script 64.1%→64.2%, workspace 54.5%→54.6%, validation 91.8%→91.7% (negligible dip)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer invoked)
- [x] All critical review-responses addressed (0 critical)
- [x] All significant review-responses addressed (5 significant, all addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

All 9 findings addressed:

- RR-5C8UG (significant, addressed) — moved `rela.write_file` to write bindings
- RR-H3AHW (significant, addressed) — panic at `NewWriter` construction on nil EntityManager
- RR-AJPCR (significant, addressed) — dataentry materializes luaWriteDeps per request
- RR-CICPB (significant, addressed) — swapped helper option precedence (context wins)
- RR-X5JP3 (significant, addressed) — dropped duplicate meta field in validation.Service
- RR-T9C62 (minor, addressed) — dropped cacheDir parameter; derived from deps.ProjectRoot
- RR-THW7A (minor, addressed) — memoised Workspace.Tracer / Searcher with sync.Once
- RR-COVP8 (nit, addressed) — named NopScriptExecutor args
- RR-XAB5E (minor, addressed) — new TestDoExecuteTask_PullsLuaWriteDeps integration test

## Acceptance Verification

- [x] Each acceptance criterion tested (see implementation checklist IMPL-1WACM)
- [x] Test evidence documented

**Acceptance Status:**

1. PASS — `lua.Services` struct deleted; `ReadDeps`/`WriteDeps` value structs in `internal/lua/deps.go`.
2. PASS — each call site uses minimal deps via `Workspace.LuaReadDeps()` / `LuaWriteDeps()` + `script.NewReaderRuntime` / `NewWriterRuntime` helpers.
3. PASS — `metamodel.ScriptContext` deleted; Engine methods take `lua.WriteDeps` + `*entity.Entity` directly.
4. PASS — `svc.Manager = nil` hack gone; mutation bindings absent on reader.
5. PASS — fallback patches deleted in executor.go, action.go, validation/lua.go.
6. PASS — `script.NewReaderRuntime` / `NewWriterRuntime` helpers exist; 6 call sites use them.
7. PASS — `go-arch-lint` contract preserved; no new package-cycle violations.
8. PASS — `just test` passes across all 39 packages.
9. PASS — no `interface{}` type-assertions for workspace-to-lua handoff (grep confirms).
10. PASS — `TestReaderRuntime_MutationCallIsLuaNilCall` verifies the Lua-level nil-call error.

## Documentation (enhancements only)

Skipped — internal refactor, no user-facing surface change.

- [x] ~~Docs-checklist created~~ (N/A: internal refactor, no docs needed)

## Final Checks

- [x] Commit messages explain why, not just what (two commits: initial refactor + review-fix-up)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: user will create PR manually)
- [x] ~~All CI checks pass~~ (deferred: will be verified on PR)
- [x] ~~PR URL documented below~~ (deferred: no PR yet)

**PR:** *deferred — branch `TKT-SNG55-lua-deps-refactor` ready for PR creation*
